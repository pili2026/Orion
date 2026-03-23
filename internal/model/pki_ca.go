package model

import (
	"time"

	"github.com/google/uuid"
)

// PKICA stores the single root Certificate Authority used to sign all
// gateway client certificates. Only one active CA is expected at a time;
// the table is bootstrapped on first use by PKIService.
type PKICA struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	CertPEM   string    `gorm:"type:text;not null"                              json:"-"`
	KeyPEM    string    `gorm:"type:text;not null"                              json:"-"`
	ExpiresAt time.Time `                                                       json:"expires_at"`
	CreatedAt time.Time `                                                       json:"created_at"`
}
