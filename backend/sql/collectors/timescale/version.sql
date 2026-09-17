-- TimescaleDB extension version facts (read-only).
SELECT
  current_database() AS database_name,
  e.extname AS extension_name,
  e.extversion AS extension_version,
  n.nspname AS schema_name
FROM pg_extension e
JOIN pg_namespace n ON n.oid = e.extnamespace
WHERE e.extname = 'timescaledb';
