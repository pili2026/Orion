package model

import (
	"github.com/google/uuid"
)

// Site represents a physical facility or installation site.
type Site struct {
	BaseModel
	UtilityID string `gorm:"uniqueIndex;type:varchar(100);not null" json:"utility_id"`
	NameCN    string `gorm:"type:varchar(100);not null"             json:"name_cn"`
	SiteCode  string `gorm:"uniqueIndex;type:varchar(100);not null" json:"site_code"`

	// Relationships
	Zones    []Zone    `gorm:"foreignKey:SiteID" json:"zones,omitempty"`
	Gateways []Gateway `gorm:"foreignKey:SiteID" json:"gateways,omitempty"`
}

// Zone is a logical grouping within a site used for dashboard UI organisation.
// Each Zone may optionally be tied 1:1 to a Gateway via GatewayID (enforced by
// a partial unique index in the DB). A nil GatewayID means the zone is a
// site-level grouping not associated with a specific gateway.
type Zone struct {
	BaseModel
	SiteID       uuid.UUID  `gorm:"type:uuid;not null;index"               json:"site_id"`
	GatewayID    *uuid.UUID `gorm:"type:uuid;uniqueIndex;default:null"      json:"gateway_id,omitempty"`
	ZoneName     string     `gorm:"type:varchar(100);not null"             json:"zone_name"`
	DisplayOrder int        `gorm:"type:int;not null;default:0"            json:"display_order"`

	// Relationships
	Site    *Site    `gorm:"foreignKey:SiteID"    json:"site,omitempty"`
	Gateway *Gateway `gorm:"foreignKey:GatewayID" json:"gateway,omitempty"`
	Devices []Device `gorm:"foreignKey:ZoneID"    json:"devices,omitempty"`
}
