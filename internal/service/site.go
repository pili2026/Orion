package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/hill/orion/internal/dto"
	"github.com/hill/orion/internal/model"
	"github.com/hill/orion/internal/repository"
)

// ── Site ──────────────────────────────────────────────────────────────────────

type SiteService interface {
	List(ctx context.Context) ([]dto.SiteResponse, error)
	GetByID(ctx context.Context, id uuid.UUID) (*dto.SiteResponse, error)
	Create(ctx context.Context, req dto.CreateSiteRequest) (*dto.SiteResponse, error)
	Update(ctx context.Context, id uuid.UUID, req dto.UpdateSiteRequest) (*dto.SiteResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type siteService struct {
	repo repository.SiteRepository
}

func NewSiteService(repo repository.SiteRepository) SiteService {
	return &siteService{repo: repo}
}

func (s *siteService) List(ctx context.Context) ([]dto.SiteResponse, error) {
	sites, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dto.SiteResponse, len(sites))
	for i, site := range sites {
		out[i] = toSiteResponse(site)
	}
	return out, nil
}

func (s *siteService) GetByID(ctx context.Context, id uuid.UUID) (*dto.SiteResponse, error) {
	site, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	resp := toSiteResponse(*site)
	return &resp, nil
}

func (s *siteService) Create(ctx context.Context, req dto.CreateSiteRequest) (*dto.SiteResponse, error) {
	site := &model.Site{
		UtilityID: req.UtilityID,
		NameCN:    req.NameCN,
		SiteCode:  req.SiteCode,
	}
	if err := s.repo.Create(ctx, site); err != nil {
		return nil, fmt.Errorf("create site: %w", err)
	}
	resp := toSiteResponse(*site)
	return &resp, nil
}

func (s *siteService) Update(ctx context.Context, id uuid.UUID, req dto.UpdateSiteRequest) (*dto.SiteResponse, error) {
	site := &model.Site{}
	site.ID = id
	if req.NameCN != nil {
		site.NameCN = *req.NameCN
	}
	if req.SiteCode != nil {
		site.SiteCode = *req.SiteCode
	}
	if err := s.repo.Update(ctx, site); err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id)
}

func (s *siteService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func toSiteResponse(site model.Site) dto.SiteResponse {
	return dto.SiteResponse{
		ID:        site.ID,
		UtilityID: site.UtilityID,
		NameCN:    site.NameCN,
		SiteCode:  site.SiteCode,
		CreatedAt: site.CreatedAt,
		UpdatedAt: site.UpdatedAt,
	}
}

// ── Zone ──────────────────────────────────────────────────────────────────────

type ZoneService interface {
	List(ctx context.Context, siteID uuid.UUID) ([]dto.ZoneResponse, error)
	Create(ctx context.Context, siteID uuid.UUID, req dto.CreateZoneRequest) (*dto.ZoneResponse, error)
	Update(ctx context.Context, siteID, zoneID uuid.UUID, req dto.UpdateZoneRequest) (*dto.ZoneResponse, error)
	Delete(ctx context.Context, siteID, zoneID uuid.UUID) error
}

type zoneService struct {
	repo     repository.ZoneRepository
	siteRepo repository.SiteRepository
}

func NewZoneService(repo repository.ZoneRepository, siteRepo repository.SiteRepository) ZoneService {
	return &zoneService{repo: repo, siteRepo: siteRepo}
}

func (s *zoneService) List(ctx context.Context, siteID uuid.UUID) ([]dto.ZoneResponse, error) {
	// Verify site exists
	if _, err := s.siteRepo.GetByID(ctx, siteID); err != nil {
		return nil, err
	}
	zones, err := s.repo.ListBySite(ctx, siteID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.ZoneResponse, len(zones))
	for i, z := range zones {
		out[i] = toZoneResponse(z)
	}
	return out, nil
}

func (s *zoneService) Create(ctx context.Context, siteID uuid.UUID, req dto.CreateZoneRequest) (*dto.ZoneResponse, error) {
	// Verify site exists
	if _, err := s.siteRepo.GetByID(ctx, siteID); err != nil {
		return nil, err
	}
	zone := &model.Zone{
		SiteID:       siteID,
		ZoneName:     req.ZoneName,
		DisplayOrder: req.DisplayOrder,
	}
	if err := s.repo.Create(ctx, zone); err != nil {
		return nil, fmt.Errorf("create zone: %w", err)
	}
	resp := toZoneResponse(*zone)
	return &resp, nil
}

func (s *zoneService) Update(ctx context.Context, siteID, zoneID uuid.UUID, req dto.UpdateZoneRequest) (*dto.ZoneResponse, error) {
	zone := &model.Zone{}
	zone.ID = zoneID
	zone.SiteID = siteID
	if req.ZoneName != nil {
		zone.ZoneName = *req.ZoneName
	}
	if req.DisplayOrder != nil {
		zone.DisplayOrder = *req.DisplayOrder
	}
	if err := s.repo.Update(ctx, zone); err != nil {
		return nil, err
	}
	updated, err := s.repo.GetByID(ctx, siteID, zoneID)
	if err != nil {
		return nil, err
	}
	resp := toZoneResponse(*updated)
	return &resp, nil
}

func (s *zoneService) Delete(ctx context.Context, siteID, zoneID uuid.UUID) error {
	return s.repo.Delete(ctx, siteID, zoneID)
}

func toZoneResponse(zone model.Zone) dto.ZoneResponse {
	return dto.ZoneResponse{
		ID:           zone.ID,
		SiteID:       zone.SiteID,
		ZoneName:     zone.ZoneName,
		DisplayOrder: zone.DisplayOrder,
		CreatedAt:    zone.CreatedAt,
		UpdatedAt:    zone.UpdatedAt,
	}
}

// isSiteNotFound checks for either site or zone not found errors.
func isSiteNotFound(err error) bool {
	return errors.Is(err, repository.ErrSiteNotFound)
}
