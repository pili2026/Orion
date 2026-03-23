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

// ErrGatewayNotFound is returned when a gateway lookup yields no result.
// It wraps apperr.ErrNotFound so handlers can use errors.Is(err, apperr.ErrNotFound).
var ErrGatewayNotFound = fmt.Errorf("gateway not found: %w", apperr.ErrNotFound)

// GatewayRepository defines the data-access contract for gateways.
type GatewayRepository interface {
	Create(ctx context.Context, gw *model.Gateway) error
	// List returns all non-deleted gateways. When siteID is non-nil the result is
	// filtered to that site only.
	List(ctx context.Context, siteID *uuid.UUID) ([]model.Gateway, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Gateway, error)
	Update(ctx context.Context, gw *model.Gateway) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type gatewayRepository struct {
	db *gorm.DB
}

// NewGatewayRepository creates a new GatewayRepository backed by GORM.
func NewGatewayRepository(db *gorm.DB) GatewayRepository {
	return &gatewayRepository{db: db}
}

func (r *gatewayRepository) Create(ctx context.Context, gw *model.Gateway) error {
	if err := r.db.WithContext(ctx).Create(gw).Error; err != nil {
		return fmt.Errorf("create gateway: %w", err)
	}
	return nil
}

func (r *gatewayRepository) List(ctx context.Context, siteID *uuid.UUID) ([]model.Gateway, error) {
	var gateways []model.Gateway
	q := r.db.WithContext(ctx)
	if siteID != nil {
		q = q.Where("site_id = ?", *siteID)
	}
	if err := q.Find(&gateways).Error; err != nil {
		return nil, fmt.Errorf("list gateways: %w", err)
	}
	return gateways, nil
}

func (r *gatewayRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Gateway, error) {
	var gw model.Gateway
	err := r.db.WithContext(ctx).First(&gw, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrGatewayNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get gateway by id: %w", err)
	}
	return &gw, nil
}

// Update persists only the non-zero fields supplied in gw.
// Using Model+Updates (rather than Save) avoids accidentally blanking
// columns that were not included in the PATCH payload.
func (r *gatewayRepository) Update(ctx context.Context, gw *model.Gateway) error {
	result := r.db.WithContext(ctx).Model(gw).Updates(gw)
	if result.Error != nil {
		return fmt.Errorf("update gateway: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrGatewayNotFound
	}
	return nil
}

// Delete performs a GORM soft-delete (sets deleted_at).
func (r *gatewayRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&model.Gateway{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("delete gateway: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrGatewayNotFound
	}
	return nil
}
