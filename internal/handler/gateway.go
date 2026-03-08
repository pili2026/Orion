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
	"github.com/hill/orion/internal/repository"
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
	List(ctx context.Context) ([]dto.GatewayResponse, error)
	GetByID(ctx context.Context, id uuid.UUID) (*dto.GatewayResponse, error)
	Update(ctx context.Context, id uuid.UUID, req dto.UpdateGatewayRequest) (*dto.GatewayResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
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
func (h *Handler) ListGateways(c *gin.Context) {
	gateways, err := h.GatewaySvc.List(c.Request.Context())
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
	if errors.Is(err, repository.ErrGatewayNotFound) {
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
	if errors.Is(err, repository.ErrGatewayNotFound) {
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
	if errors.Is(err, repository.ErrGatewayNotFound) {
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
