package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/hill/orion/internal/dto"
	"github.com/hill/orion/pkg/apperr"
)

// DeviceService is the interface the device handler depends on.
type DeviceService interface {
	Update(ctx context.Context, id uuid.UUID, req dto.UpdateDeviceRequest) (*dto.DeviceResponse, error)
}

// UpdateDevice handles PATCH /api/v1/devices/:id.
// Currently supports updating display_name only.
func (h *Handler) UpdateDevice(c *gin.Context) {
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}

	var req dto.UpdateDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	dev, err := h.DeviceSvc.Update(c.Request.Context(), id, req)
	if errors.Is(err, apperr.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}
	if err != nil {
		slog.Error("UpdateDevice failed", slog.String("id", id.String()), slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update device"})
		return
	}

	c.JSON(http.StatusOK, dev)
}
