package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/hill/orion/internal/dto"
	"github.com/hill/orion/internal/model"
	"github.com/hill/orion/internal/repository"
)

// DeviceService handles business logic for device management.
type DeviceService interface {
	Update(ctx context.Context, id uuid.UUID, req dto.UpdateDeviceRequest) (*dto.DeviceResponse, error)
}

type deviceService struct {
	repo repository.DeviceRepository
}

// NewDeviceService creates a new DeviceService.
func NewDeviceService(repo repository.DeviceRepository) DeviceService {
	return &deviceService{repo: repo}
}

// Update applies a partial update to the device and returns the updated record.
func (s *deviceService) Update(ctx context.Context, id uuid.UUID, req dto.UpdateDeviceRequest) (*dto.DeviceResponse, error) {
	// Verify the device exists first.
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		return nil, err
	}

	updates := map[string]any{}
	if req.DisplayName != nil {
		updates["display_name"] = *req.DisplayName
	}

	if len(updates) > 0 {
		if err := s.repo.Update(ctx, id, updates); err != nil {
			return nil, fmt.Errorf("update device: %w", err)
		}
	}

	// Re-fetch to return the current state.
	dev, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	resp := toDeviceResponse(dev)
	return &resp, nil
}

// toDeviceResponse maps a model.Device to its DTO representation.
func toDeviceResponse(dev *model.Device) dto.DeviceResponse {
	return dto.DeviceResponse{
		ID:             dev.ID.String(),
		GatewayID:      dev.GatewayID.String(),
		ZoneID:         dev.ZoneID.String(),
		DeviceTypeCode: dev.DeviceTypeCode,
		FuncTag:        dev.FuncTag,
		DisplayName:    dev.DisplayName,
		DeviceCode:     dev.DeviceCode,
		CreatedAt:      dev.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      dev.UpdatedAt.Format(time.RFC3339),
	}
}
