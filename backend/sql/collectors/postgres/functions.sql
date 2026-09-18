-- postgres.functions — inventory of functions/procedures/aggregates.
-- pg_get_functiondef is not valid for aggregates (prokind = 'a').
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
  CASE
    WHEN p.prokind = 'a' THEN NULL
    ELSE pg_catalog.pg_get_functiondef(p.oid)
  END AS function_definition
FROM pg_proc p
JOIN pg_namespace n ON n.oid = p.pronamespace
JOIN pg_language l ON l.oid = p.prolang
WHERE n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
  AND n.nspname <> 'information_schema'
ORDER BY n.nspname, p.proname, identity_arguments;
