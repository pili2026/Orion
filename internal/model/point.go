package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// PhysicalPoint represents the actual physical point
// (e.g., sensor or actuator) on a device,
//
//	which can be assigned to logical points in different zones with metadata.
type PhysicalPoint struct {
	BaseModel
	DeviceID  uuid.UUID `gorm:"type:uuid;index" json:"device_id"`
	PortIndex int       `gorm:"type:int" json:"port_index"`

	// Relationships
	Device           *Device           `gorm:"foreignKey:DeviceID" json:"device,omitempty"`
	PointAssignments []PointAssignment `gorm:"foreignKey:PointID" json:"point_assignments,omitempty"`
}

// PointAssignment logical point configuration (includes JSONB and validity period)
type PointAssignment struct {
	BaseModel
	PointID        uuid.UUID      `gorm:"type:uuid;index" json:"point_id"`
	ZoneID         uuid.UUID      `gorm:"type:uuid;index" json:"zone_id"`
	SensorTypeCode string         `gorm:"type:varchar(50);index" json:"sensor_type_code"` // FK to a SensorType dictionary table (not defined here)
	FuncTag        string         `gorm:"type:varchar(100)" json:"func_tag"`
	SensorName     string         `gorm:"type:varchar(100)" json:"sensor_name"`
	Unit           string         `gorm:"type:varchar(20)" json:"unit"`
	Metadata       datatypes.JSON `gorm:"type:jsonb" json:"metadata"`
	ActiveFrom     *time.Time     `json:"active_from"`
	ActiveTo       *time.Time     `json:"active_to"`

	// Relationships
	PhysicalPoint *PhysicalPoint `gorm:"foreignKey:PointID" json:"physical_point,omitempty"`
	Zone          *Zone          `gorm:"foreignKey:ZoneID" json:"zone,omitempty"`
}
