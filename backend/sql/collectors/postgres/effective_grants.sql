-- Effective table privileges for login roles; metadata only, no credentials.
SELECT current_database(), n.nspname, c.relname, r.rolname,
  array_remove(ARRAY[
    CASE WHEN has_table_privilege(r.oid,c.oid,'SELECT') THEN 'SELECT' END,
    CASE WHEN has_table_privilege(r.oid,c.oid,'INSERT') THEN 'INSERT' END,
    CASE WHEN has_table_privilege(r.oid,c.oid,'UPDATE') THEN 'UPDATE' END,
    CASE WHEN has_table_privilege(r.oid,c.oid,'DELETE') THEN 'DELETE' END,
    CASE WHEN has_table_privilege(r.oid,c.oid,'TRUNCATE') THEN 'TRUNCATE' END,
    CASE WHEN has_table_privilege(r.oid,c.oid,'REFERENCES') THEN 'REFERENCES' END,
    CASE WHEN has_table_privilege(r.oid,c.oid,'TRIGGER') THEN 'TRIGGER' END
  ]::text[],NULL)::text[] AS privileges
FROM pg_class c
JOIN pg_namespace n ON n.oid=c.relnamespace
CROSS JOIN pg_roles r
WHERE c.relkind IN ('r','p','f') AND r.rolcanlogin
  AND n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
  AND n.nspname <> 'information_schema'
ORDER BY n.nspname,c.relname,r.rolname;
