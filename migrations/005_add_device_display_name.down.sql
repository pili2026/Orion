-- Migration: 005_add_device_display_name.down.sql

ALTER TABLE devices
    DROP COLUMN IF EXISTS display_name;
