-- Migration: 001_init_schema.down.sql
-- Drops all tables in reverse dependency order.

DROP TABLE IF EXISTS telemetry_sensors;
DROP TABLE IF EXISTS telemetry_flow_meters;
DROP TABLE IF EXISTS telemetry_inverters;
DROP TABLE IF EXISTS telemetry_meters;
DROP TABLE IF EXISTS point_assignments;
DROP TABLE IF EXISTS physical_points;
DROP TABLE IF EXISTS devices;
DROP TABLE IF EXISTS gateways;
DROP TABLE IF EXISTS zones;
DROP TABLE IF EXISTS sites;
DROP TABLE IF EXISTS device_types;