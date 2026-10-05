package postgres

const sequencesSQL = `
SELECT
  current_database() AS database_name,
  n.nspname AS schema_name,
  c.relname AS sequence_name,
  format_type(s.seqtypid, NULL) AS data_type,
  s.seqstart::text AS start_value,
  s.seqincrement::text AS increment_by,
  s.seqmax::text AS max_value,
  s.seqmin::text AS min_value,
  s.seqcycle AS cycle,
  CASE WHEN t.relname IS NOT NULL THEN tn.nspname || '.' || t.relname ELSE NULL END AS owned_by_table,
  a.attname AS owned_by_column,
  ps.last_value::text AS last_value
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
JOIN pg_sequence s ON s.seqrelid = c.oid
LEFT JOIN pg_sequences ps ON ps.schemaname = n.nspname AND ps.sequencename = c.relname
LEFT JOIN pg_depend d
  ON d.objid = c.oid AND d.deptype IN ('a', 'i') AND d.classid = 'pg_class'::regclass
LEFT JOIN pg_class t ON t.oid = d.refobjid
LEFT JOIN pg_namespace tn ON tn.oid = t.relnamespace
LEFT JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = d.refobjsubid
WHERE c.relkind = 'S'
  AND n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
  AND n.nspname <> 'information_schema'
ORDER BY n.nspname, c.relname
`

const triggersSQL = `
SELECT
  current_database() AS database_name,
  n.nspname AS schema_name,
  c.relname AS table_name,
  t.tgname AS trigger_name,
  CASE t.tgenabled
    WHEN 'O' THEN 'enabled'
    WHEN 'D' THEN 'disabled'
    WHEN 'R' THEN 'replica'
    WHEN 'A' THEN 'always'
    ELSE t.tgenabled::text
  END AS enabled,
  CASE
    WHEN (t.tgtype & 2) = 2 THEN 'BEFORE'
    WHEN (t.tgtype & 64) = 64 THEN 'INSTEAD OF'
    ELSE 'AFTER'
  END AS timing,
  array_to_string(ARRAY[
    CASE WHEN (t.tgtype & 4) = 4 THEN 'INSERT' END,
    CASE WHEN (t.tgtype & 8) = 8 THEN 'DELETE' END,
    CASE WHEN (t.tgtype & 16) = 16 THEN 'UPDATE' END,
    CASE WHEN (t.tgtype & 32) = 32 THEN 'TRUNCATE' END
  ]::text[], ', ') AS event_manipulation,
  pg_get_triggerdef(t.oid, true) AS action_statement
FROM pg_trigger t
JOIN pg_class c ON c.oid = t.tgrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE NOT t.tgisinternal
  AND c.relkind IN ('r', 'p')
  AND n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
  AND n.nspname <> 'information_schema'
ORDER BY n.nspname, c.relname, t.tgname
`

const policiesSQL = `
SELECT
  current_database() AS database_name,
  n.nspname AS schema_name,
  c.relname AS table_name,
  p.polname AS policy_name,
  CASE WHEN p.polpermissive THEN 'PERMISSIVE' ELSE 'RESTRICTIVE' END AS permissive,
  COALESCE((
    SELECT array_agg(pr.rolname::text ORDER BY pr.rolname)
    FROM pg_roles pr
    WHERE pr.oid = ANY (p.polroles)
  ), ARRAY[]::text[]) AS roles,
  CASE p.polcmd
    WHEN 'r' THEN 'SELECT'
    WHEN 'a' THEN 'INSERT'
    WHEN 'w' THEN 'UPDATE'
    WHEN 'd' THEN 'DELETE'
    WHEN '*' THEN 'ALL'
    ELSE p.polcmd::text
  END AS cmd,
  pg_get_expr(p.polqual, p.polrelid) AS qual,
  pg_get_expr(p.polwithcheck, p.polrelid) AS with_check
FROM pg_policy p
JOIN pg_class c ON c.oid = p.polrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
  AND n.nspname <> 'information_schema'
ORDER BY n.nspname, c.relname, p.polname
`
