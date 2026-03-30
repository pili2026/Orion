-- Migration: 008_zone_gateway_id.up.sql
-- Adds gateway_id to zones so each Zone can be tied 1:1 to a Gateway.
-- NULL is allowed for zones that represent a site-level logical grouping
-- not associated with any specific gateway (e.g. manually created zones).

ALTER TABLE zones ADD COLUMN gateway_id UUID REFERENCES gateways(id);

-- Partial unique index: at most one non-deleted zone per gateway.
-- Allows gateway_id = NULL for zones not tied to a gateway.
CREATE UNIQUE INDEX idx_zones_gateway_id
    ON zones (gateway_id)
    WHERE deleted_at IS NULL;
