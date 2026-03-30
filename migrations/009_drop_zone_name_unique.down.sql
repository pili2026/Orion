-- Migration: 009_drop_zone_name_unique.down.sql
ALTER TABLE zones ADD CONSTRAINT zones_site_id_zone_name_key
    UNIQUE (site_id, zone_name);
