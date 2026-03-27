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

var ErrSiteNotFound = fmt.Errorf("site not found: %w", apperr.ErrNotFound)
var ErrZoneNotFound = fmt.Errorf("zone not found: %w", apperr.ErrNotFound)

// ── Site ──────────────────────────────────────────────────────────────────────

type SiteRepository interface {
	List(ctx context.Context) ([]model.Site, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Site, error)
	Create(ctx context.Context, site *model.Site) error
	Update(ctx context.Context, site *model.Site) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type siteRepository struct{ db *gorm.DB }

func NewSiteRepository(db *gorm.DB) SiteRepository {
	return &siteRepository{db: db}
}

func (r *siteRepository) List(ctx context.Context) ([]model.Site, error) {
	var sites []model.Site
	if err := r.db.WithContext(ctx).Find(&sites).Error; err != nil {
		return nil, fmt.Errorf("list sites: %w", err)
	}
	return sites, nil
}

func (r *siteRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Site, error) {
	var site model.Site
	err := r.db.WithContext(ctx).First(&site, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSiteNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get site: %w", err)
	}
	return &site, nil
}

func (r *siteRepository) Create(ctx context.Context, site *model.Site) error {
	if err := r.db.WithContext(ctx).Create(site).Error; err != nil {
		return fmt.Errorf("create site: %w", err)
	}
	return nil
}

func (r *siteRepository) Update(ctx context.Context, site *model.Site) error {
	result := r.db.WithContext(ctx).Model(site).Updates(site)
	if result.Error != nil {
		return fmt.Errorf("update site: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrSiteNotFound
	}
	return nil
}

func (r *siteRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&model.Site{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("delete site: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrSiteNotFound
	}
	return nil
}

// ── Zone ──────────────────────────────────────────────────────────────────────

type ZoneRepository interface {
	ListBySite(ctx context.Context, siteID uuid.UUID) ([]model.Zone, error)
	GetByID(ctx context.Context, siteID, zoneID uuid.UUID) (*model.Zone, error)
	// GetByGatewayID returns the Zone associated with the given gateway, or
	// ErrZoneNotFound if no zone with that gateway_id exists.
	GetByGatewayID(ctx context.Context, gatewayID uuid.UUID) (*model.Zone, error)
	Create(ctx context.Context, zone *model.Zone) error
	Update(ctx context.Context, zone *model.Zone) error
	Delete(ctx context.Context, siteID, zoneID uuid.UUID) error
}

type zoneRepository struct{ db *gorm.DB }

func NewZoneRepository(db *gorm.DB) ZoneRepository {
	return &zoneRepository{db: db}
}

func (r *zoneRepository) ListBySite(ctx context.Context, siteID uuid.UUID) ([]model.Zone, error) {
	var zones []model.Zone
	err := r.db.WithContext(ctx).
		Where("site_id = ?", siteID).
		Order("display_order ASC, zone_name ASC").
		Find(&zones).Error
	if err != nil {
		return nil, fmt.Errorf("list zones: %w", err)
	}
	return zones, nil
}

func (r *zoneRepository) GetByID(ctx context.Context, siteID, zoneID uuid.UUID) (*model.Zone, error) {
	var zone model.Zone
	err := r.db.WithContext(ctx).
		First(&zone, "id = ? AND site_id = ?", zoneID, siteID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrZoneNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get zone: %w", err)
	}
	return &zone, nil
}

func (r *zoneRepository) GetByGatewayID(ctx context.Context, gatewayID uuid.UUID) (*model.Zone, error) {
	var zone model.Zone
	err := r.db.WithContext(ctx).
		First(&zone, "gateway_id = ?", gatewayID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrZoneNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get zone by gateway_id: %w", err)
	}
	return &zone, nil
}

func (r *zoneRepository) Create(ctx context.Context, zone *model.Zone) error {
	if err := r.db.WithContext(ctx).Create(zone).Error; err != nil {
		return fmt.Errorf("create zone: %w", err)
	}
	return nil
}

func (r *zoneRepository) Update(ctx context.Context, zone *model.Zone) error {
	result := r.db.WithContext(ctx).Model(zone).Updates(zone)
	if result.Error != nil {
		return fmt.Errorf("update zone: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrZoneNotFound
	}
	return nil
}

func (r *zoneRepository) Delete(ctx context.Context, siteID, zoneID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Delete(&model.Zone{}, "id = ? AND site_id = ?", zoneID, siteID)
	if result.Error != nil {
		return fmt.Errorf("delete zone: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrZoneNotFound
	}
	return nil
}
