-- Executed by internal/collectors/postgres. Keep byte-for-byte SQL aligned with the Go collector.
SELECT current_database(), n.nspname, c.relname, a.attname,
  CASE WHEN acl.grantee = 0 THEN '' ELSE g.rolname END AS grantee,
  acl.privilege_type
FROM pg_catalog.pg_attribute a
JOIN pg_catalog.pg_class c ON c.oid = a.attrelid AND c.relkind IN ('r','p','f','v','m')
JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
CROSS JOIN LATERAL aclexplode(a.attacl) AS acl
LEFT JOIN pg_catalog.pg_roles g ON g.oid = acl.grantee
WHERE a.attnum > 0 AND NOT a.attisdropped AND a.attacl IS NOT NULL
  AND n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
  AND n.nspname <> 'information_schema'
ORDER BY n.nspname, c.relname, a.attname, 5, acl.privilege_type
