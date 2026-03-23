-- Migration: 006_pki_certs.down.sql

ALTER TABLE gateways
    DROP COLUMN IF EXISTS cert_status,
    DROP COLUMN IF EXISTS cert_issued_at,
    DROP COLUMN IF EXISTS cert_expires_at,
    DROP COLUMN IF EXISTS cert_serial,
    DROP COLUMN IF EXISTS client_cert_pem,
    DROP COLUMN IF EXISTS client_key_pem;

DROP TABLE IF EXISTS pki_ca;
