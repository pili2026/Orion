package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
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
}

// NewGatewayService creates a new GatewayService.
func NewGatewayService(repo repository.GatewayRepository, dynsec *DynsecService) *GatewayService {
	return &GatewayService{
		repo:   repo,
		dynsec: dynsec,
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

// List returns all non-deleted gateways.
func (s *GatewayService) List(ctx context.Context) ([]dto.GatewayResponse, error) {
	gateways, err := s.repo.List(ctx)
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
		CreatedAt:     gw.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     gw.UpdatedAt.Format(time.RFC3339),
	}

	if gw.LastSeenAt != nil {
		t := gw.LastSeenAt.Format(time.RFC3339)
		resp.LastSeenAt = &t
	}

	return resp
}
