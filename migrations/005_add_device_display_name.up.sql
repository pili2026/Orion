-- Migration: 005_add_device_display_name.up.sql
-- Adds an optional human-readable display name to the devices table.
-- NULL means not set; front-end should fall back to func_tag.

ALTER TABLE devices
    ADD COLUMN IF NOT EXISTS display_name VARCHAR(100);
