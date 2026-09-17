-- Extension inventory facts for the current database (read-only).
SELECT
  current_database() AS database_name,
  e.extname AS extension_name,
  e.extversion AS extension_version,
  n.nspname AS schema_name,
  e.extrelocatable AS is_relocatable
FROM pg_extension e
JOIN pg_namespace n ON n.oid = e.extnamespace
ORDER BY e.extname;
