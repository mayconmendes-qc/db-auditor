package postgres

// SQL kept in sync with backend/sql/collectors/postgres/*.sql (source of truth for review).

const serverSQL = `
SELECT
  current_setting('server_version') AS server_version,
  pg_postmaster_start_time() AS started_at,
  (now() - pg_postmaster_start_time())::text AS uptime,
  current_setting('TimeZone') AS timezone,
  current_setting('server_encoding') AS server_encoding,
  current_setting('max_connections')::int AS max_connections,
  (SELECT count(*) FROM pg_stat_activity) AS current_connections,
  current_setting('autovacuum', true) AS autovacuum
`

const databasesSQL = `
SELECT
  d.datname AS database_name,
  pg_catalog.pg_get_userbyid(d.datdba) AS owner_name,
  pg_catalog.pg_encoding_to_char(d.encoding) AS encoding,
  d.datcollate AS collate_name,
  d.datctype AS ctype_name,
  d.datallowconn AS allow_connections,
  d.datistemplate AS is_template,
  COALESCE(pg_database_size(d.datname), 0) AS size_bytes,
  COALESCE(s.numbackends, 0) AS connection_count
FROM pg_database d
LEFT JOIN pg_stat_database s ON s.datname = d.datname
ORDER BY d.datname
`

const schemasSQL = `
SELECT
  current_database() AS database_name,
  n.nspname AS schema_name,
  pg_catalog.pg_get_userbyid(n.nspowner) AS owner_name,
  COALESCE(c.table_count, 0) AS table_count,
  COALESCE(c.view_count, 0) AS view_count,
  COALESCE(c.matview_count, 0) AS materialized_view_count,
  COALESCE(c.sequence_count, 0) AS sequence_count,
  COALESCE(f.function_count, 0) AS function_count,
  COALESCE((
    SELECT sum(pg_total_relation_size(cl.oid))
    FROM pg_class cl
    WHERE cl.relnamespace = n.oid
      AND cl.relkind IN ('r', 'm', 'i', 'S', 't')
  ), 0) AS size_bytes
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
ORDER BY n.nspname
`
