-- Table inventory facts for the current database (read-only).
-- Sizes are always bytes; stats come from pg_stat_user_tables when available.
SELECT
  current_database() AS database_name,
  n.nspname AS schema_name,
  c.relname AS table_name,
  pg_catalog.pg_get_userbyid(c.relowner) AS owner_name,
  c.relkind::text AS relkind,
  COALESCE(pg_relation_size(c.oid), 0) AS data_size_bytes,
  COALESCE(pg_indexes_size(c.oid), 0) AS index_size_bytes,
  COALESCE(pg_total_relation_size(c.oid), 0) AS total_size_bytes,
  COALESCE(c.reltuples, 0)::bigint AS row_estimate,
  COALESCE(s.n_live_tup, 0)::bigint AS n_live_tup,
  COALESCE(s.n_dead_tup, 0)::bigint AS n_dead_tup,
  COALESCE(s.n_tup_ins, 0)::bigint AS n_tup_ins,
  COALESCE(s.n_tup_upd, 0)::bigint AS n_tup_upd,
  COALESCE(s.n_tup_del, 0)::bigint AS n_tup_del,
  COALESCE(s.seq_scan, 0)::bigint AS seq_scan,
  COALESCE(s.idx_scan, 0)::bigint AS idx_scan,
  s.last_vacuum,
  s.last_autovacuum,
  s.last_analyze,
  s.last_autoanalyze,
  (
    SELECT count(*)
    FROM pg_attribute a
    WHERE a.attrelid = c.oid
      AND a.attnum > 0
      AND NOT a.attisdropped
  )::int AS column_count,
  EXISTS (
    SELECT 1
    FROM pg_index i
    WHERE i.indrelid = c.oid AND i.indisprimary
  ) AS has_primary_key
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
LEFT JOIN pg_stat_user_tables s
  ON s.relid = c.oid
WHERE c.relkind = 'r'
  AND n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
  AND n.nspname <> 'information_schema'
ORDER BY n.nspname, c.relname;
