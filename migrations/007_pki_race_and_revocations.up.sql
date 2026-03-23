-- Migration: 007_pki_race_and_revocations.up.sql

-- Fix 1: enforce at-most-one active CA at the DB level.
-- The singleton column acts as a sentinel: UNIQUE(singleton=TRUE) means
-- only one row can ever satisfy the constraint, so concurrent INSERTs from
-- multiple replicas will result in exactly one winner and N-1 silent no-ops.
ALTER TABLE pki_ca
    ADD COLUMN IF NOT EXISTS singleton BOOLEAN NOT NULL DEFAULT TRUE;
CREATE UNIQUE INDEX IF NOT EXISTS idx_pki_ca_singleton ON pki_ca (singleton);

-- Fix 2: audit trail for revoked client certificates.
-- Serials recorded here are candidates for CRL / OCSP enforcement once
-- the Talos edge agent supports certificate-based MQTT authentication checks.
CREATE TABLE IF NOT EXISTS revoked_cert_serials (
    id          UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    gateway_id  UUID         NOT NULL REFERENCES gateways (id),
    cert_serial VARCHAR(100) NOT NULL,
    revoked_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    reason      TEXT
);
CREATE INDEX IF NOT EXISTS idx_revoked_cert_serials_gateway_id ON revoked_cert_serials (gateway_id);
