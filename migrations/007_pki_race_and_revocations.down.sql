-- Migration: 007_pki_race_and_revocations.down.sql

DROP TABLE IF EXISTS revoked_cert_serials;

DROP INDEX IF EXISTS idx_pki_ca_singleton;
ALTER TABLE pki_ca DROP COLUMN IF EXISTS singleton;
