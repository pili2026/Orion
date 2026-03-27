-- Migration: 008_zone_gateway_id.down.sql

DROP INDEX IF EXISTS idx_zones_gateway_id;
ALTER TABLE zones DROP COLUMN IF EXISTS gateway_id;
