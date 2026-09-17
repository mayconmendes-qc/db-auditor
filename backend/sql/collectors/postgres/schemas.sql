-- Schema inventory facts for the current database (read-only).
SELECT
  current_database() AS database_name,
  n.nspname AS schema_name,
  pg_catalog.pg_get_userbyid(n.nspowner) AS owner_name,
  COALESCE(c.table_count, 0) AS table_count,
  COALESCE(c.view_count, 0) AS view_count,
  COALESCE(c.matview_count, 0) AS materialized_view_count,
  COALESCE(c.sequence_count, 0) AS sequence_count,
  COALESCE(f.function_count, 0) AS function_count,
  COALESCE(pg_catalog.pg_namespace_size(n.oid), 0) AS size_bytes
FROM pg_namespace n
LEFT JOIN LATERAL (
  SELECT
    count(*) FILTER (WHERE c.relkind = 'r') AS table_count,
    count(*) FILTER (WHERE c.relkind = 'v') AS view_count,
    count(*) FILTER (WHERE c.relkind = 'm') AS matview_count,
    count(*) FILTER (WHERE c.relkind = 'S') AS sequence_count
  FROM pg_class c
  WHERE c.relnamespace = n.oid
    AND c.relkind IN ('r', 'v', 'm', 'S')
) c ON true
LEFT JOIN LATERAL (
  SELECT count(*) AS function_count
  FROM pg_proc p
  WHERE p.pronamespace = n.oid
) f ON true
WHERE n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
ORDER BY n.nspname;
