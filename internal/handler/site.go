package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/hill/orion/internal/dto"
	"github.com/hill/orion/internal/repository"
)

// SiteService is the interface the site handler depends on.
type SiteService interface {
	List(ctx context.Context) ([]dto.SiteResponse, error)
	GetByID(ctx context.Context, id uuid.UUID) (*dto.SiteResponse, error)
	Create(ctx context.Context, req dto.CreateSiteRequest) (*dto.SiteResponse, error)
	Update(ctx context.Context, id uuid.UUID, req dto.UpdateSiteRequest) (*dto.SiteResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// ZoneService is the interface the zone handler depends on.
type ZoneService interface {
	List(ctx context.Context, siteID uuid.UUID) ([]dto.ZoneResponse, error)
	Create(ctx context.Context, siteID uuid.UUID, req dto.CreateZoneRequest) (*dto.ZoneResponse, error)
	Update(ctx context.Context, siteID, zoneID uuid.UUID, req dto.UpdateZoneRequest) (*dto.ZoneResponse, error)
	Delete(ctx context.Context, siteID, zoneID uuid.UUID) error
}

// ── Site handlers ─────────────────────────────────────────────────────────────

func (h *Handler) ListSites(c *gin.Context) {
	sites, err := h.SiteSvc.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list sites"})
		return
	}
	c.JSON(http.StatusOK, sites)
}

func (h *Handler) GetSite(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid site id"})
		return
	}
	site, err := h.SiteSvc.GetByID(c.Request.Context(), id)
	if errors.Is(err, repository.ErrSiteNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "site not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get site"})
		return
	}
	c.JSON(http.StatusOK, site)
}

func (h *Handler) CreateSite(c *gin.Context) {
	var req dto.CreateSiteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	site, err := h.SiteSvc.Create(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create site"})
		return
	}
	c.JSON(http.StatusCreated, site)
}

func (h *Handler) UpdateSite(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid site id"})
		return
	}
	var req dto.UpdateSiteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	site, err := h.SiteSvc.Update(c.Request.Context(), id, req)
	if errors.Is(err, repository.ErrSiteNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "site not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update site"})
		return
	}
	c.JSON(http.StatusOK, site)
}

func (h *Handler) DeleteSite(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid site id"})
		return
	}
	err = h.SiteSvc.Delete(c.Request.Context(), id)
	if errors.Is(err, repository.ErrSiteNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "site not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete site"})
		return
	}
	c.Status(http.StatusNoContent)
}

// ── Zone handlers ─────────────────────────────────────────────────────────────

func (h *Handler) ListZones(c *gin.Context) {
	siteID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid site id"})
		return
	}
	zones, err := h.ZoneSvc.List(c.Request.Context(), siteID)
	if errors.Is(err, repository.ErrSiteNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "site not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list zones"})
		return
	}
	c.JSON(http.StatusOK, zones)
}

func (h *Handler) CreateZone(c *gin.Context) {
	siteID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid site id"})
		return
	}
	var req dto.CreateZoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	zone, err := h.ZoneSvc.Create(c.Request.Context(), siteID, req)
	if errors.Is(err, repository.ErrSiteNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "site not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create zone"})
		return
	}
	c.JSON(http.StatusCreated, zone)
}

func (h *Handler) UpdateZone(c *gin.Context) {
	siteID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid site id"})
		return
	}
	zoneID, err := uuid.Parse(c.Param("zone_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid zone id"})
		return
	}
	var req dto.UpdateZoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	zone, err := h.ZoneSvc.Update(c.Request.Context(), siteID, zoneID, req)
	if errors.Is(err, repository.ErrZoneNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "zone not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update zone"})
		return
	}
	c.JSON(http.StatusOK, zone)
}

func (h *Handler) DeleteZone(c *gin.Context) {
	siteID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid site id"})
		return
	}
	zoneID, err := uuid.Parse(c.Param("zone_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid zone id"})
		return
	}
	err = h.ZoneSvc.Delete(c.Request.Context(), siteID, zoneID)
	if errors.Is(err, repository.ErrZoneNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "zone not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete zone"})
		return
	}
	c.Status(http.StatusNoContent)
}
