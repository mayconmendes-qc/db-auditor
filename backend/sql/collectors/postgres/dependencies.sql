-- Catalog-tracked view/matview and function dependencies; dynamic SQL is not inferred.
SELECT DISTINCT current_database(), source_ns.nspname, source.relname,
  CASE source.relkind WHEN 'm' THEN 'materialized_view' ELSE 'view' END,
  target_ns.nspname, target.relname,
  CASE target.relkind WHEN 'm' THEN 'materialized_view' WHEN 'v' THEN 'view' ELSE 'table' END
FROM pg_depend d
JOIN pg_rewrite rw ON d.classid='pg_rewrite'::regclass AND d.objid=rw.oid
JOIN pg_class source ON source.oid=rw.ev_class AND source.relkind IN ('v','m')
JOIN pg_namespace source_ns ON source_ns.oid=source.relnamespace
JOIN pg_class target ON d.refclassid='pg_class'::regclass AND d.refobjid=target.oid
JOIN pg_namespace target_ns ON target_ns.oid=target.relnamespace
WHERE source.oid<>target.oid
  AND source_ns.nspname NOT LIKE 'pg\_%' ESCAPE '\'
  AND target_ns.nspname NOT LIKE 'pg\_%' ESCAPE '\'
  AND source_ns.nspname <> 'information_schema'
  AND target_ns.nspname <> 'information_schema'
UNION
SELECT DISTINCT current_database(), source_ns.nspname,
  p.proname || '(' || pg_get_function_identity_arguments(p.oid) || ')', 'function',
  target_ns.nspname, target.relname,
  CASE target.relkind WHEN 'm' THEN 'materialized_view' WHEN 'v' THEN 'view' ELSE 'table' END
FROM pg_depend d
JOIN pg_proc p ON d.classid='pg_proc'::regclass AND d.objid=p.oid
JOIN pg_namespace source_ns ON source_ns.oid=p.pronamespace
JOIN pg_class target ON d.refclassid='pg_class'::regclass AND d.refobjid=target.oid
JOIN pg_namespace target_ns ON target_ns.oid=target.relnamespace
WHERE source_ns.nspname NOT LIKE 'pg\_%' ESCAPE '\'
  AND target_ns.nspname NOT LIKE 'pg\_%' ESCAPE '\'
  AND source_ns.nspname <> 'information_schema'
  AND target_ns.nspname <> 'information_schema';
