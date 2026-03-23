package model

import (
	"time"

	"github.com/google/uuid"
)

// PKICA stores the single root Certificate Authority used to sign all
// gateway client certificates. Only one active CA is expected at a time;
// the table is bootstrapped on first use by PKIService.
//
// The Singleton column (always TRUE) carries a UNIQUE index so that
// concurrent INSERTs from multiple replicas resolve to exactly one winner
// at the DB level — no application-layer locking required.
type PKICA struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	CertPEM   string    `gorm:"type:text;not null"                              json:"-"`
	KeyPEM    string    `gorm:"type:text;not null"                              json:"-"`
	ExpiresAt time.Time `                                                       json:"expires_at"`
	CreatedAt time.Time `                                                       json:"created_at"`
	// Singleton is always TRUE; the UNIQUE index on this column enforces
	// the at-most-one-CA invariant at the database level.
	Singleton bool `gorm:"not null;default:true" json:"-"`
}
