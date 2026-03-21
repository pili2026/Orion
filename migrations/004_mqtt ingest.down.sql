-- migrations/004_mqtt_ingest.down.sql
--
-- Reverses 004_mqtt_ingest.up.sql.
-- NOTE: INTEGER → TEXT conversion is lossless (0 → "0", NULL stays NULL).

-- ── 2. Revert telemetry_inverters.error / alert  INTEGER → TEXT ──────────────

ALTER TABLE telemetry_inverters
    ALTER COLUMN error TYPE TEXT USING error::text,
    ALTER COLUMN alert TYPE TEXT USING alert::text;

-- ── 1. Remove received_at from all telemetry hypertables ─────────────────────

ALTER TABLE telemetry_meters      DROP COLUMN IF EXISTS received_at;
ALTER TABLE telemetry_inverters   DROP COLUMN IF EXISTS received_at;
ALTER TABLE telemetry_flow_meters DROP COLUMN IF EXISTS received_at;
ALTER TABLE telemetry_sensors     DROP COLUMN IF EXISTS received_at;