-- Function and procedure inventory facts (read-only).
SELECT
  current_database() AS database_name,
  n.nspname AS schema_name,
  p.proname AS function_name,
  pg_catalog.pg_get_function_identity_arguments(p.oid) AS identity_arguments,
  pg_catalog.pg_get_userbyid(p.proowner) AS owner_name,
  l.lanname AS language_name,
  p.prosecdef AS is_security_definer,
  p.provolatile::text AS volatility,
  p.proparallel::text AS parallel_safety,
  p.prokind::text AS kind,
  pg_catalog.pg_get_functiondef(p.oid) AS function_definition
FROM pg_proc p
JOIN pg_namespace n ON n.oid = p.pronamespace
JOIN pg_language l ON l.oid = p.prolang
WHERE n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
  AND n.nspname <> 'information_schema'
ORDER BY n.nspname, p.proname, identity_arguments;
