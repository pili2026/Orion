package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/hill/orion/internal/model"
	"github.com/hill/orion/pkg/apperr"
)

// ErrDeviceNotFound is returned when a device lookup yields no result.
var ErrDeviceNotFound = fmt.Errorf("device not found: %w", apperr.ErrNotFound)

// DeviceRepository defines the data-access contract for devices.
type DeviceRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Device, error)
	// Update applies the given column→value map to the device with the given ID.
	// Using a map (rather than a struct) ensures zero-value strings are saved correctly.
	Update(ctx context.Context, id uuid.UUID, updates map[string]any) error
}

type deviceRepository struct {
	db *gorm.DB
}

// NewDeviceRepository creates a new DeviceRepository backed by GORM.
func NewDeviceRepository(db *gorm.DB) DeviceRepository {
	return &deviceRepository{db: db}
}

func (r *deviceRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Device, error) {
	var dev model.Device
	err := r.db.WithContext(ctx).First(&dev, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrDeviceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get device by id: %w", err)
	}
	return &dev, nil
}

func (r *deviceRepository) Update(ctx context.Context, id uuid.UUID, updates map[string]any) error {
	result := r.db.WithContext(ctx).
		Model(&model.Device{}).
		Where("id = ?", id).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update device: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrDeviceNotFound
	}
	return nil
}
