package model

import (
	"time"

	"github.com/google/uuid"
)

// Gateway represents an Edge device (Talos) that connects to the MQTT broker
// and manages one or more downstream Modbus devices.
//
// Certificate state machine: etl_synced → cert_issued → mqtt_pending → mqtt_connected
type Gateway struct {
	BaseModel
	SiteID        uuid.UUID  `gorm:"type:uuid;not null;index"                          json:"site_id"`
	SerialNo      string     `gorm:"uniqueIndex;type:varchar(100);not null"             json:"serial_no"`
	Mac           string     `gorm:"uniqueIndex;type:varchar(50);not null"              json:"mac"`
	Model         string     `gorm:"type:varchar(50);not null"                          json:"model"`
	DisplayName   string     `gorm:"type:varchar(100);not null"                         json:"display_name"`
	Status        string     `gorm:"type:varchar(50);not null;default:'offline'"        json:"status"`
	NetworkStatus string     `gorm:"type:varchar(50);not null;default:'offline'"        json:"network_status"`
	SSHPort       int        `gorm:"type:int"                                           json:"ssh_port"`
	MQTTUsername  string     `gorm:"type:varchar(100);not null"                         json:"mqtt_username"`
	LastSeenAt    *time.Time `                                                          json:"last_seen_at"`

	// PKI / certificate fields
	CertStatus    string     `gorm:"type:varchar(50);not null;default:'etl_synced'"     json:"cert_status"`
	CertIssuedAt  *time.Time `                                                          json:"cert_issued_at"`
	CertExpiresAt *time.Time `                                                          json:"cert_expires_at"`
	CertSerial    string     `gorm:"type:varchar(100)"                                  json:"cert_serial"`
	ClientCertPEM string     `gorm:"type:text"                                          json:"-"` // never serialised
	ClientKeyPEM  string     `gorm:"type:text"                                          json:"-"` // never serialised

	// Relationships
	Site    *Site    `gorm:"foreignKey:SiteID"   json:"site,omitempty"`
	Devices []Device `gorm:"foreignKey:GatewayID" json:"devices,omitempty"`
}
