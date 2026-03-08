-- Migration: 002_etl_tables.up.sql
-- Support tables for the ETL migration process.

-- etl_table_map: stores the resolved mapping from old table names to Orion UUIDs.
-- Populated by etl-meta, consumed by etl-telemetry.
CREATE TABLE IF NOT EXISTS etl_table_map (
    old_table_name VARCHAR(200) PRIMARY KEY,
    utility_id     VARCHAR(100) NOT NULL,
    device_type    VARCHAR(10)  NOT NULL,
    -- For CI / SE / SF: the device UUID in the devices table
    device_id      UUID,
    -- For ST / SP / SR / SO: the point_assignment UUID (used as FK in telemetry_sensors)
    assignment_id  UUID,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- etl_checkpoints: tracks per-table progress for the telemetry ETL job.
-- last_id is a TIMESTAMPTZ because the old tables use a datetime as their PK.
CREATE TABLE IF NOT EXISTS etl_checkpoints (
    old_table_name VARCHAR(200) PRIMARY KEY,
    last_id        TIMESTAMPTZ,                         -- resume from here on restart
    status         VARCHAR(20)  NOT NULL DEFAULT 'pending', -- pending|running|done|error
    rows_migrated  BIGINT       NOT NULL DEFAULT 0,
    started_at     TIMESTAMPTZ,
    finished_at    TIMESTAMPTZ,
    error_msg      TEXT,
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);