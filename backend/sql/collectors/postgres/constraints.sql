-- Constraint inventory facts for the current database (read-only).
SELECT
  current_database() AS database_name,
  n.nspname AS schema_name,
  rel.relname AS table_name,
  c.conname AS constraint_name,
  c.contype::text AS constraint_type,
  pg_catalog.pg_get_constraintdef(c.oid, true) AS constraint_definition,
  c.convalidated AS is_validated,
  c.condeferrable AS is_deferrable,
  c.condeferred AS is_deferred
FROM pg_constraint c
JOIN pg_class rel ON rel.oid = c.conrelid
JOIN pg_namespace n ON n.oid = rel.relnamespace
WHERE rel.relkind = 'r'
  AND n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
  AND n.nspname <> 'information_schema'
ORDER BY n.nspname, rel.relname, c.conname;
