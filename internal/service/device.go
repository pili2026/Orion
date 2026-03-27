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
	repo        repository.DeviceRepository
	gatewayRepo repository.GatewayRepository
	zoneRepo    repository.ZoneRepository
}

// NewDeviceService creates a new DeviceService.
func NewDeviceService(repo repository.DeviceRepository, gatewayRepo repository.GatewayRepository, zoneRepo repository.ZoneRepository) DeviceService {
	return &deviceService{repo: repo, gatewayRepo: gatewayRepo, zoneRepo: zoneRepo}
}

// Update applies a partial update to the device and returns the updated record.
// If zone_id is provided, it is validated to belong to the same site as the
// device's gateway before the update is applied.
func (s *deviceService) Update(ctx context.Context, id uuid.UUID, req dto.UpdateDeviceRequest) (*dto.DeviceResponse, error) {
	// Verify the device exists first.
	dev, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	updates := map[string]any{}
	if req.DisplayName != nil {
		updates["display_name"] = *req.DisplayName
	}

	if req.ZoneID != nil {
		zoneID, err := uuid.Parse(*req.ZoneID)
		if err != nil {
			return nil, fmt.Errorf("invalid zone_id: %w", err)
		}

		// Fetch the device's gateway to get the site.
		gw, err := s.gatewayRepo.GetByID(ctx, dev.GatewayID)
		if err != nil {
			return nil, fmt.Errorf("get gateway for device: %w", err)
		}

		// Validate the target zone belongs to the same site as the gateway.
		if _, err := s.zoneRepo.GetByID(ctx, gw.SiteID, zoneID); err != nil {
			return nil, fmt.Errorf("zone %s not found in site %s: %w", zoneID, gw.SiteID, err)
		}

		updates["zone_id"] = zoneID
	}

	if len(updates) > 0 {
		if err := s.repo.Update(ctx, id, updates); err != nil {
			return nil, fmt.Errorf("update device: %w", err)
		}
	}

	// Re-fetch to return the current state.
	dev, err = s.repo.GetByID(ctx, id)
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
