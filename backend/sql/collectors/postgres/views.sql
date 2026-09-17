-- View and materialized view inventory facts (read-only).
SELECT
  current_database() AS database_name,
  n.nspname AS schema_name,
  c.relname AS view_name,
  pg_catalog.pg_get_userbyid(c.relowner) AS owner_name,
  c.relkind::text AS relkind,
  pg_catalog.pg_get_viewdef(c.oid, true) AS view_definition,
  CASE
    WHEN c.relkind = 'm' THEN COALESCE(pg_total_relation_size(c.oid), 0)
    ELSE 0
  END AS size_bytes
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE c.relkind IN ('v', 'm')
  AND n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
  AND n.nspname <> 'information_schema'
ORDER BY n.nspname, c.relname;
