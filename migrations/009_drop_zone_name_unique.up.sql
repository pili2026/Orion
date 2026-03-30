-- Migration: 009_drop_zone_name_unique.up.sql
-- Drop the (site_id, zone_name) unique constraint.
-- Zone uniqueness is now enforced by idx_zones_gateway_id (migration 008):
-- each Gateway can have at most one Zone, so the name constraint is redundant
-- and prevents creating multiple same-named Zones under the same Site
-- (e.g. two Gateways in the same Site both having a "default" Zone).
ALTER TABLE zones DROP CONSTRAINT zones_site_id_zone_name_key;
