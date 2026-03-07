package model

import (
	"github.com/google/uuid"
)

// Site master table
type Site struct {
	BaseModel
	UtilityID string `gorm:"uniqueIndex;type:varchar(100)" json:"utility_id"`
	NameCN    string `gorm:"type:varchar(100)" json:"name_cn"`
	SiteCode  string `gorm:"uniqueIndex;type:varchar(100)" json:"site_code"`

	// Relationships
	Zones    []Zone    `gorm:"foreignKey:SiteID" json:"zones,omitempty"`
	Gateways []Gateway `gorm:"foreignKey:SiteID" json:"gateways,omitempty"`
}

// Zone site table can have multiple zones
// (e.g., different buildings or areas within a site)
type Zone struct {
	BaseModel
	SiteID       uuid.UUID `gorm:"type:uuid;index" json:"site_id"`
	ZoneName     string    `gorm:"type:varchar(100)" json:"zone_name"`
	DisplayOrder int       `gorm:"type:int;default:0" json:"display_order"`

	// Relationships
	Site    *Site    `gorm:"foreignKey:SiteID" json:"site,omitempty"`
	Devices []Device `gorm:"foreignKey:ZoneID" json:"devices,omitempty"`
}
