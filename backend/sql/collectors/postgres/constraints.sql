-- Executed by internal/collectors/postgres. Keep byte-for-byte SQL aligned with the Go collector.
SELECT
  current_database() AS database_name,
  n.nspname AS schema_name,
  rel.relname AS table_name,
  c.conname AS constraint_name,
  c.contype::text AS constraint_type,
  pg_catalog.pg_get_constraintdef(c.oid, true) AS constraint_definition,
  supporting_index.relname AS supporting_index,
  c.convalidated AS is_validated,
  c.condeferrable AS is_deferrable,
  c.condeferred AS is_deferred,
  COALESCE((
    SELECT array_agg(a.attname::text ORDER BY u.ord)
    FROM unnest(c.conkey) WITH ORDINALITY AS u(attnum, ord)
    JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = u.attnum
  ), ARRAY[]::text[]) AS constrained_columns,
  fn.nspname AS referenced_schema_name,
  frel.relname AS referenced_table_name,
  COALESCE((
    SELECT array_agg(a.attname::text ORDER BY u.ord)
    FROM unnest(c.confkey) WITH ORDINALITY AS u(attnum, ord)
    JOIN pg_attribute a ON a.attrelid = c.confrelid AND a.attnum = u.attnum
  ), ARRAY[]::text[]) AS referenced_columns,
  CASE c.confupdtype
    WHEN 'a' THEN 'no action'
    WHEN 'r' THEN 'restrict'
    WHEN 'c' THEN 'cascade'
    WHEN 'n' THEN 'set null'
    WHEN 'd' THEN 'set default'
    ELSE NULL
  END AS fk_update_action,
  CASE c.confdeltype
    WHEN 'a' THEN 'no action'
    WHEN 'r' THEN 'restrict'
    WHEN 'c' THEN 'cascade'
    WHEN 'n' THEN 'set null'
    WHEN 'd' THEN 'set default'
    ELSE NULL
  END AS fk_delete_action,
  CASE c.confmatchtype
    WHEN 'f' THEN 'full'
    WHEN 'p' THEN 'partial'
    WHEN 's' THEN 'simple'
    ELSE NULL
  END AS fk_match_type
FROM pg_constraint c
JOIN pg_class rel ON rel.oid = c.conrelid
JOIN pg_namespace n ON n.oid = rel.relnamespace
LEFT JOIN pg_class frel ON frel.oid = c.confrelid
LEFT JOIN pg_namespace fn ON fn.oid = frel.relnamespace
LEFT JOIN pg_class supporting_index ON supporting_index.oid = c.conindid
WHERE rel.relkind IN ('r', 'p')
  AND n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
  AND n.nspname <> 'information_schema'
ORDER BY n.nspname, rel.relname, c.conname
