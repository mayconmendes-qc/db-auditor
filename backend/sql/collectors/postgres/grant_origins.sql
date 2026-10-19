-- Executed by internal/collectors/postgres. Keep byte-for-byte SQL aligned with the Go collector.
SELECT current_database(), r.rolname,
  CASE c.relkind WHEN 'v' THEN 'view' WHEN 'm' THEN 'view' WHEN 'S' THEN 'sequence' ELSE 'table' END,
  n.nspname || '.' || c.relname, p.priv
FROM pg_catalog.pg_class c
JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
CROSS JOIN pg_catalog.pg_roles r
CROSS JOIN LATERAL unnest(
  CASE WHEN c.relkind = 'S' THEN ARRAY['SELECT','USAGE','UPDATE']::text[]
  ELSE ARRAY['SELECT','INSERT','UPDATE','DELETE','TRUNCATE','REFERENCES','TRIGGER']::text[] END
) AS p(priv)
WHERE c.relkind IN ('r','p','f','v','m','S') AND r.rolcanlogin AND r.rolname !~ '^pg_'
  AND n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
  AND n.nspname <> 'information_schema'
  AND CASE WHEN c.relkind = 'S' THEN has_sequence_privilege(r.oid, c.oid, p.priv)
    ELSE has_table_privilege(r.oid, c.oid, p.priv) END
UNION ALL
SELECT current_database(), r.rolname, 'function',
  n.nspname || '.' || p.proname || '(' || pg_catalog.pg_get_function_identity_arguments(p.oid) || ')',
  'EXECUTE'
FROM pg_catalog.pg_proc p
JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace
CROSS JOIN pg_catalog.pg_roles r
WHERE r.rolcanlogin AND r.rolname !~ '^pg_'
  AND n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
  AND n.nspname <> 'information_schema'
  AND has_function_privilege(r.oid, p.oid, 'EXECUTE')
UNION ALL
SELECT current_database(), r.rolname, 'schema', n.nspname, p.priv
FROM pg_catalog.pg_namespace n
CROSS JOIN pg_catalog.pg_roles r
CROSS JOIN LATERAL unnest(ARRAY['USAGE','CREATE']::text[]) AS p(priv)
WHERE r.rolcanlogin AND r.rolname !~ '^pg_'
  AND n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
  AND n.nspname <> 'information_schema'
  AND has_schema_privilege(r.oid, n.oid, p.priv)
ORDER BY 1, 2, 3, 4, 5
