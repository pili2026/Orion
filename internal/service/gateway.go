package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/hill/orion/internal/dto"
	"github.com/hill/orion/internal/model"
	"github.com/hill/orion/internal/repository"
)

// GatewayService handles all business logic for gateway registration.
type GatewayService struct {
	repo   repository.GatewayRepository
	dynsec *DynsecService
	pki    *PKIService
}

// NewGatewayService creates a new GatewayService.
func NewGatewayService(repo repository.GatewayRepository, dynsec *DynsecService, pki *PKIService) *GatewayService {
	return &GatewayService{
		repo:   repo,
		dynsec: dynsec,
		pki:    pki,
	}
}

// Register creates a new Gateway record and provisions its MQTT credentials.
// The generated MQTT password is returned once in the response and is never
// stored — losing it requires calling a (future) reset endpoint.
func (s *GatewayService) Register(ctx context.Context, req dto.CreateGatewayRequest) (*dto.RegisterGatewayResponse, error) {
	siteID, err := uuid.Parse(req.SiteID)
	if err != nil {
		return nil, fmt.Errorf("invalid site_id: %w", err)
	}

	// The MQTT username is the serial number — it is unique, human-readable,
	// and matches the edge_id used in MQTT topics (talos/{serial_no}/...).
	mqttUsername := req.SerialNo

	mqttPassword, err := generatePassword(24)
	if err != nil {
		return nil, fmt.Errorf("generate mqtt password: %w", err)
	}

	// 1. Persist the gateway record first.
	//    If the DB write fails we never touch MQTT, keeping both systems consistent.
	gw := &model.Gateway{
		SiteID:        siteID,
		SerialNo:      req.SerialNo,
		Mac:           req.Mac,
		Model:         req.Model,
		DisplayName:   req.DisplayName,
		Status:        "offline",
		NetworkStatus: "offline",
		SSHPort:       req.SSHPort,
		MQTTUsername:  mqttUsername,
	}

	if err := s.repo.Create(ctx, gw); err != nil {
		return nil, fmt.Errorf("register gateway: %w", err)
	}

	// 2. Create the MQTT client in Mosquitto Dynamic Security.
	//    If this fails the gateway record is already in the DB; the operator
	//    can re-provision via a future /gateways/:id/reset-mqtt endpoint.
	if err := s.dynsec.CreateEdgeClient(mqttUsername, mqttPassword); err != nil {
		return nil, fmt.Errorf("provision mqtt client: %w", err)
	}

	return &dto.RegisterGatewayResponse{
		Gateway:      toGatewayResponse(gw),
		MQTTPassword: mqttPassword,
		MQTTBroker:   os.Getenv("MQTT_BROKER"),
		MQTTPort:     os.Getenv("MQTT_PORT"),
	}, nil
}

// List returns all non-deleted gateways, optionally filtered by site_id.
func (s *GatewayService) List(ctx context.Context, siteID *uuid.UUID) ([]dto.GatewayResponse, error) {
	gateways, err := s.repo.List(ctx, siteID)
	if err != nil {
		return nil, err
	}

	responses := make([]dto.GatewayResponse, len(gateways))
	for i, gw := range gateways {
		responses[i] = toGatewayResponse(&gw)
	}
	return responses, nil
}

// GetByID returns a single gateway or repository.ErrGatewayNotFound.
func (s *GatewayService) GetByID(ctx context.Context, id uuid.UUID) (*dto.GatewayResponse, error) {
	gw, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	resp := toGatewayResponse(gw)
	return &resp, nil
}

// Update applies a partial update to the gateway.
func (s *GatewayService) Update(ctx context.Context, id uuid.UUID, req dto.UpdateGatewayRequest) (*dto.GatewayResponse, error) {
	gw, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Apply only the fields the caller explicitly provided (pointer = set).
	if req.DisplayName != nil {
		gw.DisplayName = *req.DisplayName
	}
	if req.Model != nil {
		gw.Model = *req.Model
	}
	if req.Status != nil {
		gw.Status = *req.Status
	}
	if req.NetworkStatus != nil {
		gw.NetworkStatus = *req.NetworkStatus
	}
	if req.SSHPort != nil {
		gw.SSHPort = *req.SSHPort
	}

	if err := s.repo.Update(ctx, gw); err != nil {
		return nil, err
	}

	resp := toGatewayResponse(gw)
	return &resp, nil
}

// Delete soft-deletes the gateway record and removes its MQTT credentials.
func (s *GatewayService) Delete(ctx context.Context, id uuid.UUID) error {
	gw, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Revoke MQTT access first so the device can no longer connect
	// even if the subsequent DB soft-delete somehow fails.
	if err := s.dynsec.DeleteEdgeClient(gw.MQTTUsername); err != nil {
		return fmt.Errorf("revoke mqtt client: %w", err)
	}

	return s.repo.Delete(ctx, id)
}

// IssueCert issues a new client certificate for the gateway.
// The gateway's cert_status is advanced to cert_issued.
func (s *GatewayService) IssueCert(ctx context.Context, id uuid.UUID) (*dto.GatewayResponse, error) {
	gw, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := s.pki.IssueClientCert(ctx, gw); err != nil {
		return nil, fmt.Errorf("issue cert: %w", err)
	}

	resp := toGatewayResponse(gw)
	return &resp, nil
}

// DownloadCert returns the zip bytes containing ca.crt, client.crt, client.key.
func (s *GatewayService) DownloadCert(ctx context.Context, id uuid.UUID) ([]byte, string, error) {
	gw, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, "", err
	}

	zipBytes, err := s.pki.BuildCertZip(ctx, gw)
	if err != nil {
		return nil, "", err
	}

	filename := fmt.Sprintf("certs_%s.zip", gw.SerialNo)
	return zipBytes, filename, nil
}

// RevokeCert revokes the current certificate and re-issues a fresh one.
//
// TODO(talos-integration): Talos 對接後，MQTT broker 需查詢 revoked_cert_serials
// 資料表做 CRL 驗證。目前序號僅記錄，尚未實際阻擋舊憑證連線 — 舊的 client.crt
// 在 MQTT broker 端仍有效直到自然到期（clientValidityYears）。
func (s *GatewayService) RevokeCert(ctx context.Context, id uuid.UUID) (*dto.GatewayResponse, error) {
	gw, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Persist the old serial before clearing, so the revocation record is accurate.
	oldSerial := gw.CertSerial

	// Clear existing cert fields before re-issuing.
	gw.CertStatus = CertStatusEtlSynced
	gw.ClientCertPEM = ""
	gw.ClientKeyPEM = ""
	gw.CertSerial = ""
	gw.CertIssuedAt = nil
	gw.CertExpiresAt = nil

	if err := s.pki.IssueClientCert(ctx, gw); err != nil {
		return nil, fmt.Errorf("revoke and reissue cert: %w", err)
	}

	// Write audit record for the revoked serial (non-fatal: log but don't fail
	// the whole operation if the write fails — the new cert is already issued).
	if oldSerial != "" {
		if recErr := s.pki.RecordRevocation(ctx, id, oldSerial, "revoked_by_operator"); recErr != nil {
			slog.Error("record revocation failed (non-fatal)", slog.Any("error", recErr))
		}
	}

	resp := toGatewayResponse(gw)
	return &resp, nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

// generatePassword returns a cryptographically random URL-safe string of
// approximately `n` bytes of entropy (base64-encoded length will be longer).
func generatePassword(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// toGatewayResponse maps a model.Gateway to its DTO representation.
func toGatewayResponse(gw *model.Gateway) dto.GatewayResponse {
	resp := dto.GatewayResponse{
		ID:            gw.ID.String(),
		SiteID:        gw.SiteID.String(),
		SerialNo:      gw.SerialNo,
		Mac:           gw.Mac,
		Model:         gw.Model,
		DisplayName:   gw.DisplayName,
		Status:        gw.Status,
		NetworkStatus: gw.NetworkStatus,
		SSHPort:       gw.SSHPort,
		MQTTUsername:  gw.MQTTUsername,
		CertStatus:    gw.CertStatus,
		CertSerial:    gw.CertSerial,
		CreatedAt:     gw.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     gw.UpdatedAt.Format(time.RFC3339),
	}

	if gw.LastSeenAt != nil {
		t := gw.LastSeenAt.Format(time.RFC3339)
		resp.LastSeenAt = &t
	}
	if gw.CertIssuedAt != nil {
		t := gw.CertIssuedAt.Format(time.RFC3339)
		resp.CertIssuedAt = &t
	}
	if gw.CertExpiresAt != nil {
		t := gw.CertExpiresAt.Format(time.RFC3339)
		resp.CertExpiresAt = &t
	}

	return resp
}
