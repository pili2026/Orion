package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DeviceType dictionary table (PK is the code string)
type DeviceType struct {
	Code        string         `gorm:"primaryKey;type:varchar(50)" json:"code"`
	Description string         `gorm:"type:varchar(255)" json:"description"`
	Category    string         `gorm:"type:varchar(50)" json:"category"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// Device represents the actual IoT device connected to a gateway and located in a zone.
type Device struct {
	BaseModel
	GatewayID      uuid.UUID `gorm:"type:uuid;index" json:"gateway_id"`
	ZoneID         uuid.UUID `gorm:"type:uuid;index" json:"zone_id"`
	DeviceTypeCode string    `gorm:"type:varchar(50);index" json:"device_type_code"`
	FuncTag        string    `gorm:"type:varchar(100)" json:"func_tag"`
	DeviceCode     string    `gorm:"type:varchar(100)" json:"device_code"`

	// Relationships (Belongs To & Has Many)
	Gateway        *Gateway        `gorm:"foreignKey:GatewayID" json:"gateway,omitempty"`
	Zone           *Zone           `gorm:"foreignKey:ZoneID" json:"zone,omitempty"`
	DeviceType     *DeviceType     `gorm:"foreignKey:DeviceTypeCode" json:"device_type,omitempty"`
	PhysicalPoints []PhysicalPoint `gorm:"foreignKey:DeviceID" json:"physical_points,omitempty"`
}
