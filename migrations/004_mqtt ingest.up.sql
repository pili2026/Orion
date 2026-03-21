-- migrations/004_mqtt_ingest.up.sql
--
-- Supports the MQTT real-time write path (MQTTIngestService):
--
--   1. received_at TIMESTAMPTZ — the moment Orion received the MQTT message.
--      NULL for all historical ETL rows (only the MQTT path populates this column).
--
--   2. telemetry_inverters.error / .alert TEXT → INTEGER
--      Legacy data contained only "0" (normal) and "-1" (no code).
--      "-1" is normalised to NULL. The Go layer applies the same at write time.
--
-- TimescaleDB note:
--   ALTER COLUMN TYPE is blocked on hypertables with compressed chunks.
--   Steps: remove compression policy → decompress all chunks → alter column
--          → re-enable compression → restore compression policy.

-- ── 1. Add received_at to all telemetry hypertables ──────────────────────────
-- ADD COLUMN on a hypertable is always safe (no rewrite needed).

ALTER TABLE telemetry_meters      ADD COLUMN IF NOT EXISTS received_at TIMESTAMPTZ;
ALTER TABLE telemetry_inverters   ADD COLUMN IF NOT EXISTS received_at TIMESTAMPTZ;
ALTER TABLE telemetry_flow_meters ADD COLUMN IF NOT EXISTS received_at TIMESTAMPTZ;
ALTER TABLE telemetry_sensors     ADD COLUMN IF NOT EXISTS received_at TIMESTAMPTZ;

-- ── 2. Convert telemetry_inverters.error / alert  TEXT → INTEGER ─────────────

-- Step 2a: Remove the compression policy so no new auto-compression runs
--          while we are working on the table.
SELECT remove_compression_policy('telemetry_inverters', if_exists => TRUE);

-- Step 2b: Decompress every chunk.
--          decompress_chunk() is a no-op on chunks that are already uncompressed,
--          so this is safe to run even if only some chunks are compressed.
SELECT decompress_chunk(c, if_compressed => TRUE)
FROM show_chunks('telemetry_inverters') c;

-- Step 2c: Alter the column types now that all chunks are uncompressed.
--   USING clause:
--     error::integer  — cast text to int  ("0" → 0, "-1" → -1)
--     NULLIF(..., -1) — replace -1 with NULL
ALTER TABLE telemetry_inverters
    ALTER COLUMN error TYPE INTEGER USING NULLIF(error::integer, -1),
    ALTER COLUMN alert TYPE INTEGER USING NULLIF(alert::integer, -1);

-- Step 2d: Re-enable compression with the same settings as migration 003.
ALTER TABLE telemetry_inverters SET (
    timescaledb.compress,
    timescaledb.compress_orderby   = 'ts DESC',
    timescaledb.compress_segmentby = 'device_id'
);

-- Step 2e: Restore the 30-day compression policy.
SELECT add_compression_policy('telemetry_inverters', INTERVAL '30 days', if_not_exists => TRUE);