package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/hill/orion/internal/dto"
	"github.com/hill/orion/pkg/apperr"
)

// TelemetryService is the interface the telemetry handler depends on.
type TelemetryService interface {
	LatestByDevice(ctx context.Context, deviceID uuid.UUID) (any, error)
	LatestByAssignment(ctx context.Context, assignmentID uuid.UUID) (*dto.LatestSensorResponse, error)
	LatestBySite(ctx context.Context, siteID uuid.UUID) (*dto.SiteLatestResponse, error)
	HistoryByDevice(ctx context.Context, deviceID uuid.UUID, from, to time.Time) (any, error)
	HistoryByAssignment(ctx context.Context, assignmentID uuid.UUID, from, to time.Time) ([]dto.LatestSensorResponse, error)
}

// GatewayService is the interface the handler depends on.
// Declaring it here (not in service/) keeps the dependency pointing inward
// and makes the handler trivially testable with a mock.
type GatewayService interface {
	Register(ctx context.Context, req dto.CreateGatewayRequest) (*dto.RegisterGatewayResponse, error)
	List(ctx context.Context, siteID *uuid.UUID) ([]dto.GatewayResponse, error)
	GetByID(ctx context.Context, id uuid.UUID) (*dto.GatewayResponse, error)
	Update(ctx context.Context, id uuid.UUID, req dto.UpdateGatewayRequest) (*dto.GatewayResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
	IssueCert(ctx context.Context, id uuid.UUID) (*dto.GatewayResponse, error)
	DownloadCert(ctx context.Context, id uuid.UUID) ([]byte, string, error)
	RevokeCert(ctx context.Context, id uuid.UUID) (*dto.GatewayResponse, error)
}

// RegisterGateway handles POST /api/v1/gateways.
// Returns 201 with the gateway data and the one-time MQTT password.
func (h *Handler) RegisterGateway(c *gin.Context) {
	var req dto.CreateGatewayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.GatewaySvc.Register(c.Request.Context(), req)
	if err != nil {
		slog.Error("RegisterGateway failed", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "registration failed"})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// ListGateways handles GET /api/v1/gateways.
// Accepts optional query param ?site_id=<uuid> to filter by site.
func (h *Handler) ListGateways(c *gin.Context) {
	var siteID *uuid.UUID
	if raw := c.Query("site_id"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid site_id format"})
			return
		}
		siteID = &parsed
	}

	gateways, err := h.GatewaySvc.List(c.Request.Context(), siteID)
	if err != nil {
		slog.Error("ListGateways failed", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list gateways"})
		return
	}

	c.JSON(http.StatusOK, gateways)
}

// GetGateway handles GET /api/v1/gateways/:id.
func (h *Handler) GetGateway(c *gin.Context) {
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}

	gw, err := h.GatewaySvc.GetByID(c.Request.Context(), id)
	if errors.Is(err, apperr.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "gateway not found"})
		return
	}
	if err != nil {
		slog.Error("GetGateway failed", slog.String("id", id.String()), slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get gateway"})
		return
	}

	c.JSON(http.StatusOK, gw)
}

// UpdateGateway handles PATCH /api/v1/gateways/:id.
func (h *Handler) UpdateGateway(c *gin.Context) {
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}

	var req dto.UpdateGatewayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	gw, err := h.GatewaySvc.Update(c.Request.Context(), id, req)
	if errors.Is(err, apperr.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "gateway not found"})
		return
	}
	if err != nil {
		slog.Error("UpdateGateway failed", slog.String("id", id.String()), slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update gateway"})
		return
	}

	c.JSON(http.StatusOK, gw)
}

// DeleteGateway handles DELETE /api/v1/gateways/:id.
// Soft-deletes the DB record and revokes MQTT credentials.
func (h *Handler) DeleteGateway(c *gin.Context) {
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}

	err := h.GatewaySvc.Delete(c.Request.Context(), id)
	if errors.Is(err, apperr.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "gateway not found"})
		return
	}
	if err != nil {
		slog.Error("DeleteGateway failed", slog.String("id", id.String()), slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete gateway"})
		return
	}

	c.Status(http.StatusNoContent)
}

// IssueCert handles POST /api/v1/gateways/:id/issue-cert.
// Generates a new client certificate and advances cert_status to cert_issued.
func (h *Handler) IssueCert(c *gin.Context) {
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}

	gw, err := h.GatewaySvc.IssueCert(c.Request.Context(), id)
	if errors.Is(err, apperr.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "gateway not found"})
		return
	}
	if err != nil {
		slog.Error("IssueCert failed", slog.String("id", id.String()), slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to issue certificate"})
		return
	}

	c.JSON(http.StatusOK, gw)
}

// DownloadCert handles GET /api/v1/gateways/:id/download-cert.
// Returns a zip archive containing ca.crt, client.crt and client.key.
func (h *Handler) DownloadCert(c *gin.Context) {
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}

	zipBytes, filename, err := h.GatewaySvc.DownloadCert(c.Request.Context(), id)
	if errors.Is(err, apperr.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "gateway not found"})
		return
	}
	if err != nil {
		slog.Error("DownloadCert failed", slog.String("id", id.String()), slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build certificate package"})
		return
	}

	c.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	c.Data(http.StatusOK, "application/zip", zipBytes)
}

// RevokeCert handles POST /api/v1/gateways/:id/revoke-cert.
// Invalidates the current certificate and immediately re-issues a new one.
func (h *Handler) RevokeCert(c *gin.Context) {
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}

	gw, err := h.GatewaySvc.RevokeCert(c.Request.Context(), id)
	if errors.Is(err, apperr.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "gateway not found"})
		return
	}
	if err != nil {
		slog.Error("RevokeCert failed", slog.String("id", id.String()), slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke certificate"})
		return
	}

	c.JSON(http.StatusOK, gw)
}

// parseUUID extracts and validates a UUID path parameter.
// Writes a 400 response and returns false on failure.
func parseUUID(c *gin.Context, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(param))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid " + param + " format"})
		return uuid.Nil, false
	}
	return id, true
}
