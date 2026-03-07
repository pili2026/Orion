-- Migration: 001_init_schema.up.sql
-- Creates all metadata tables for the Orion platform.

-- ── Extensions ───────────────────────────────────────────────────────────────
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- ── Dictionary layer ─────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS device_types (
    code        VARCHAR(50)  PRIMARY KEY,
    description VARCHAR(255) NOT NULL,
    category    VARCHAR(50)  NOT NULL CHECK (category IN ('Gateway', 'Device', 'Sensor')),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);

-- ── Metadata layer ────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS sites (
    id         UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    utility_id VARCHAR(100) NOT NULL UNIQUE,
    name_cn    VARCHAR(100) NOT NULL,
    site_code  VARCHAR(100) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_sites_deleted_at ON sites (deleted_at);

CREATE TABLE IF NOT EXISTS zones (
    id            UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    site_id       UUID         NOT NULL REFERENCES sites (id),
    zone_name     VARCHAR(100) NOT NULL,
    display_order INTEGER      NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ,
    UNIQUE (site_id, zone_name)
);
CREATE INDEX IF NOT EXISTS idx_zones_site_id    ON zones (site_id);
CREATE INDEX IF NOT EXISTS idx_zones_deleted_at ON zones (deleted_at);

CREATE TABLE IF NOT EXISTS gateways (
    id             UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    site_id        UUID         NOT NULL REFERENCES sites (id),
    serial_no      VARCHAR(100) NOT NULL UNIQUE,
    mac            VARCHAR(50)  NOT NULL UNIQUE,
    model          VARCHAR(50)  NOT NULL,
    display_name   VARCHAR(100) NOT NULL,
    status         VARCHAR(50)  NOT NULL DEFAULT 'offline',
    network_status VARCHAR(50)  NOT NULL DEFAULT 'offline',
    ssh_port       INTEGER,
    mqtt_username  VARCHAR(100) NOT NULL,
    last_seen_at   TIMESTAMPTZ,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_gateways_site_id    ON gateways (site_id);
CREATE INDEX IF NOT EXISTS idx_gateways_deleted_at ON gateways (deleted_at);

CREATE TABLE IF NOT EXISTS devices (
    id               UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    gateway_id       UUID         NOT NULL REFERENCES gateways (id),
    zone_id          UUID         REFERENCES zones (id),
    device_type_code VARCHAR(50)  NOT NULL REFERENCES device_types (code),
    func_tag         VARCHAR(100),
    device_code      VARCHAR(100) NOT NULL,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at       TIMESTAMPTZ,
    UNIQUE (gateway_id, device_code)
);
CREATE INDEX IF NOT EXISTS idx_devices_gateway_id      ON devices (gateway_id);
CREATE INDEX IF NOT EXISTS idx_devices_zone_id         ON devices (zone_id);
CREATE INDEX IF NOT EXISTS idx_devices_device_type     ON devices (device_type_code);
CREATE INDEX IF NOT EXISTS idx_devices_deleted_at      ON devices (deleted_at);

CREATE TABLE IF NOT EXISTS physical_points (
    id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    device_id  UUID        NOT NULL REFERENCES devices (id),
    port_index INTEGER     NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    UNIQUE (device_id, port_index)
);
CREATE INDEX IF NOT EXISTS idx_physical_points_device_id  ON physical_points (device_id);
CREATE INDEX IF NOT EXISTS idx_physical_points_deleted_at ON physical_points (deleted_at);

CREATE TABLE IF NOT EXISTS point_assignments (
    id               UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    point_id         UUID         NOT NULL REFERENCES physical_points (id),
    zone_id          UUID         REFERENCES zones (id),
    sensor_type_code VARCHAR(50)  NOT NULL REFERENCES device_types (code),
    func_tag         VARCHAR(100),
    sensor_name      VARCHAR(100) NOT NULL,
    unit             VARCHAR(20),
    metadata         JSONB,
    active_from      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    active_to        TIMESTAMPTZ,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at       TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_point_assignments_point_id   ON point_assignments (point_id);
CREATE INDEX IF NOT EXISTS idx_point_assignments_zone_id    ON point_assignments (zone_id);
CREATE INDEX IF NOT EXISTS idx_point_assignments_deleted_at ON point_assignments (deleted_at);

-- ── Telemetry layer (TimescaleDB hypertables) ─────────────────────────────────

CREATE TABLE IF NOT EXISTS telemetry_meters (
    ts        TIMESTAMPTZ NOT NULL,
    device_id UUID        NOT NULL,
    voltage   NUMERIC,
    current   NUMERIC,
    kw        NUMERIC,
    kva       NUMERIC,
    kvar      NUMERIC,
    kwh       NUMERIC,
    kvah      NUMERIC,
    kvarh     NUMERIC,
    current_a NUMERIC,
    current_b NUMERIC,
    current_c NUMERIC,
    pf        NUMERIC,
    status    TEXT
);
SELECT create_hypertable('telemetry_meters', 'ts', if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS idx_telemetry_meters_device ON telemetry_meters (device_id, ts DESC);

CREATE TABLE IF NOT EXISTS telemetry_inverters (
    ts        TIMESTAMPTZ NOT NULL,
    device_id UUID        NOT NULL,
    voltage   NUMERIC,
    current   NUMERIC,
    kw        NUMERIC,
    kwh       NUMERIC,
    hz        NUMERIC,
    error     TEXT,
    alert     TEXT,
    invstatus TEXT,
    status    TEXT
);
SELECT create_hypertable('telemetry_inverters', 'ts', if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS idx_telemetry_inverters_device ON telemetry_inverters (device_id, ts DESC);

CREATE TABLE IF NOT EXISTS telemetry_flow_meters (
    ts            TIMESTAMPTZ NOT NULL,
    device_id     UUID        NOT NULL,
    flow          NUMERIC,
    consumption   NUMERIC,
    revconsumption NUMERIC,
    direction     INTEGER,
    status        TEXT
);
SELECT create_hypertable('telemetry_flow_meters', 'ts', if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS idx_telemetry_flow_meters_device ON telemetry_flow_meters (device_id, ts DESC);

CREATE TABLE IF NOT EXISTS telemetry_sensors (
    ts            TIMESTAMPTZ NOT NULL,
    assignment_id UUID        NOT NULL,
    val           NUMERIC,
    status        TEXT
);
SELECT create_hypertable('telemetry_sensors', 'ts', if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS idx_telemetry_sensors_assignment ON telemetry_sensors (assignment_id, ts DESC);