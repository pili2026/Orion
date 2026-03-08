-- migrations/003_hypertables.up.sql
--
-- Converts telemetry tables into TimescaleDB hypertables and configures
-- compression.  Retention is intentionally omitted (keep data forever).
--
-- Chunk interval: 1 week
--   At 1 row/min/device this yields ~10,080 rows per device per chunk,
--   which fits comfortably in memory and gives good query performance.
--
-- Compression: enabled after 30 days
--   Older chunks are compressed automatically, typically saving ~90% space.
--   Compressed chunks are still fully queryable.

-- ── telemetry_meters (SE - Power Meters) ─────────────────────────────────────
SELECT create_hypertable(
    'telemetry_meters', 'ts',
    chunk_time_interval => INTERVAL '1 week',
    if_not_exists => TRUE
);

ALTER TABLE telemetry_meters SET (
    timescaledb.compress,
    timescaledb.compress_orderby   = 'ts DESC',
    timescaledb.compress_segmentby = 'device_id'
);

SELECT add_compression_policy(
    'telemetry_meters',
    INTERVAL '30 days',
    if_not_exists => TRUE
);

-- ── telemetry_inverters (CI - Inverters) ──────────────────────────────────────
SELECT create_hypertable(
    'telemetry_inverters', 'ts',
    chunk_time_interval => INTERVAL '1 week',
    if_not_exists => TRUE
);

ALTER TABLE telemetry_inverters SET (
    timescaledb.compress,
    timescaledb.compress_orderby   = 'ts DESC',
    timescaledb.compress_segmentby = 'device_id'
);

SELECT add_compression_policy(
    'telemetry_inverters',
    INTERVAL '30 days',
    if_not_exists => TRUE
);

-- ── telemetry_flow_meters (SF - Flow Meters) ──────────────────────────────────
SELECT create_hypertable(
    'telemetry_flow_meters', 'ts',
    chunk_time_interval => INTERVAL '1 week',
    if_not_exists => TRUE
);

ALTER TABLE telemetry_flow_meters SET (
    timescaledb.compress,
    timescaledb.compress_orderby   = 'ts DESC',
    timescaledb.compress_segmentby = 'device_id'
);

SELECT add_compression_policy(
    'telemetry_flow_meters',
    INTERVAL '30 days',
    if_not_exists => TRUE
);

-- ── telemetry_sensors (ST/SP/SR/SO - Universal narrow table) ─────────────────
SELECT create_hypertable(
    'telemetry_sensors', 'ts',
    chunk_time_interval => INTERVAL '1 week',
    if_not_exists => TRUE
);

ALTER TABLE telemetry_sensors SET (
    timescaledb.compress,
    timescaledb.compress_orderby   = 'ts DESC',
    timescaledb.compress_segmentby = 'assignment_id'
);

SELECT add_compression_policy(
    'telemetry_sensors',
    INTERVAL '30 days',
    if_not_exists => TRUE
);