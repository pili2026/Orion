package model

import (
	"time"

	"github.com/google/uuid"
)

// Gateway represents the edge device that connects to the MQTT broker and manages multiple devices.
type Gateway struct {
	BaseModel
	SiteID        uuid.UUID  `gorm:"type:uuid;index" json:"site_id"`
	SerialNo      string     `gorm:"uniqueIndex;type:varchar(100)" json:"serial_no"`
	Mac           string     `gorm:"uniqueIndex;type:varchar(50)" json:"mac"`
	Model         string     `gorm:"type:varchar(50)" json:"model"`
	DisplayName   string     `gorm:"type:varchar(100)" json:"display_name"`
	Status        string     `gorm:"type:varchar(50)" json:"status"`
	NetworkStatus string     `gorm:"type:varchar(50)" json:"network_status"`
	SSHPort       int        `gorm:"type:int" json:"ssh_port"`
	MQTTUsername  string     `gorm:"type:varchar(100)" json:"mqtt_username"`
	LastSeenAt    *time.Time `json:"last_seen_at"`

	// Relationships (Belongs To & Has Many)
	Site    *Site    `gorm:"foreignKey:SiteID" json:"site,omitempty"`
	Devices []Device `gorm:"foreignKey:GatewayID" json:"devices,omitempty"`
}
