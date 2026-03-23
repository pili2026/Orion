package model

import (
	"time"

	"github.com/google/uuid"
)

// RevokedCertSerial is an audit record written whenever a gateway's client
// certificate is revoked via RevokeCert.
//
// TODO(talos-integration): once the Talos edge agent supports mTLS with the
// MQTT broker, the broker's auth plugin should query this table to implement
// a Certificate Revocation List (CRL). Currently serials are recorded here
// but do NOT actively block the old certificate from connecting — the old
// cert remains valid at the broker until its natural expiry (clientValidityYears).
type RevokedCertSerial struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	GatewayID  uuid.UUID `gorm:"type:uuid;not null;index"`
	CertSerial string    `gorm:"type:varchar(100);not null"`
	RevokedAt  time.Time `gorm:"not null;default:now()"`
	Reason     string    `gorm:"type:text"`
}
