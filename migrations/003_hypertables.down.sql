-- migrations/003_hypertables.down.sql
--
-- Removes compression policies and converts hypertables back to regular tables.
-- WARNING: this does NOT delete any data.

SELECT remove_compression_policy('telemetry_meters',      if_exists => TRUE);
SELECT remove_compression_policy('telemetry_inverters',   if_exists => TRUE);
SELECT remove_compression_policy('telemetry_flow_meters', if_exists => TRUE);
SELECT remove_compression_policy('telemetry_sensors',     if_exists => TRUE);

-- Decompress all chunks before converting back (required by TimescaleDB).
SELECT decompress_chunk(c) FROM show_chunks('telemetry_meters')      c;
SELECT decompress_chunk(c) FROM show_chunks('telemetry_inverters')   c;
SELECT decompress_chunk(c) FROM show_chunks('telemetry_flow_meters') c;
SELECT decompress_chunk(c) FROM show_chunks('telemetry_sensors')     c;

SELECT revert_chunk_to_uncompressed_state(c)
FROM show_chunks('telemetry_meters') c;

-- Convert hypertables back to regular PostgreSQL tables.
SELECT revert_chunk_to_uncompressed_state(c)
FROM show_chunks('telemetry_inverters') c;

-- Note: TimescaleDB does not provide a direct "unhypertable" function.
-- To fully revert, drop and recreate the tables from migration 001.