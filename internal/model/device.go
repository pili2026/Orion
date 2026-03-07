package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DeviceType is a dictionary / lookup table keyed by a short code string.
type DeviceType struct {
	Code        string         `gorm:"primaryKey;type:varchar(50)"        json:"code"`
	Description string         `gorm:"type:varchar(255);not null"         json:"description"`
	Category    string         `gorm:"type:varchar(50);not null"          json:"category"`
	CreatedAt   time.Time      `                                          json:"created_at"`
	UpdatedAt   time.Time      `                                          json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index"                              json:"-"`
}

// Device represents a physical Modbus device connected to a Gateway.
type Device struct {
	BaseModel
	GatewayID      uuid.UUID `gorm:"type:uuid;not null;index"           json:"gateway_id"`
	ZoneID         uuid.UUID `gorm:"type:uuid;index"                    json:"zone_id"`
	DeviceTypeCode string    `gorm:"type:varchar(50);not null;index"    json:"device_type_code"`
	FuncTag        string    `gorm:"type:varchar(100)"                  json:"func_tag"`
	DeviceCode     string    `gorm:"type:varchar(100);not null"         json:"device_code"`

	// Relationships
	Gateway        *Gateway        `gorm:"foreignKey:GatewayID"    json:"gateway,omitempty"`
	Zone           *Zone           `gorm:"foreignKey:ZoneID"       json:"zone,omitempty"`
	DeviceType     *DeviceType     `gorm:"foreignKey:DeviceTypeCode" json:"device_type,omitempty"`
	PhysicalPoints []PhysicalPoint `gorm:"foreignKey:DeviceID"     json:"physical_points,omitempty"`
}
