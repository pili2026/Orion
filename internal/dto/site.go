package dto

import (
	"time"

	"github.com/google/uuid"
)

// SiteResponse is the API response for GET /sites and GET /sites/:id.
type SiteResponse struct {
	ID        uuid.UUID `json:"id"`
	UtilityID string    `json:"utility_id"`
	NameCN    string    `json:"name_cn"`
	SiteCode  string    `json:"site_code"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ── Site CRUD ─────────────────────────────────────────────────────────────────

type CreateSiteRequest struct {
	UtilityID string `json:"utility_id" binding:"required"`
	NameCN    string `json:"name_cn"    binding:"required"`
	SiteCode  string `json:"site_code"  binding:"required"`
}

type UpdateSiteRequest struct {
	NameCN   *string `json:"name_cn"`
	SiteCode *string `json:"site_code"`
}

// ── Zone CRUD ─────────────────────────────────────────────────────────────────

type ZoneResponse struct {
	ID           uuid.UUID  `json:"id"`
	SiteID       uuid.UUID  `json:"site_id"`
	GatewayID    *uuid.UUID `json:"gateway_id,omitempty"`
	ZoneName     string     `json:"zone_name"`
	DisplayOrder int        `json:"display_order"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type CreateZoneRequest struct {
	GatewayID    *string `json:"gateway_id"`
	ZoneName     string  `json:"zone_name"     binding:"required"`
	DisplayOrder int     `json:"display_order"`
}

type UpdateZoneRequest struct {
	GatewayID    *string `json:"gateway_id"`
	ZoneName     *string `json:"zone_name"`
	DisplayOrder *int    `json:"display_order"`
}
