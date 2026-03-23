-- Migration: 006_pki_certs.up.sql
-- Adds PKI certificate infrastructure for gateway mTLS provisioning.
-- State machine: etl_synced → cert_issued → mqtt_pending → mqtt_connected

-- ── CA storage ────────────────────────────────────────────────────────────────
-- Stores the single root CA used to sign all gateway client certificates.
-- The private key is stored as PEM; in production this should be encrypted
-- at rest (e.g. using a KMS-backed column).
CREATE TABLE IF NOT EXISTS pki_ca (
    id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    cert_pem   TEXT        NOT NULL,
    key_pem    TEXT        NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── Gateway PKI columns ───────────────────────────────────────────────────────
ALTER TABLE gateways
    ADD COLUMN IF NOT EXISTS cert_status     VARCHAR(50)  NOT NULL DEFAULT 'etl_synced',
    ADD COLUMN IF NOT EXISTS cert_issued_at  TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS cert_expires_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS cert_serial     VARCHAR(100),
    ADD COLUMN IF NOT EXISTS client_cert_pem TEXT,
    ADD COLUMN IF NOT EXISTS client_key_pem  TEXT;
