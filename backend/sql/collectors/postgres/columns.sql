-- Column inventory facts for the current database (read-only).
SELECT
  current_database() AS database_name,
  n.nspname AS schema_name,
  c.relname AS table_name,
  a.attname AS column_name,
  a.attnum AS ordinal_position,
  pg_catalog.format_type(a.atttypid, a.atttypmod) AS data_type,
  NOT a.attnotnull AS is_nullable,
  pg_catalog.pg_get_expr(ad.adbin, ad.adrelid) AS column_default,
  a.attgenerated <> '' AS is_generated,
  CASE
    WHEN a.attidentity = 'a' THEN 'always'
    WHEN a.attidentity = 'd' THEN 'by default'
    ELSE NULL
  END AS identity_generation,
  col.collname AS collation_name
FROM pg_attribute a
JOIN pg_class c ON c.oid = a.attrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
LEFT JOIN pg_attrdef ad ON ad.adrelid = a.attrelid AND ad.adnum = a.attnum
LEFT JOIN pg_collation col ON col.oid = a.attcollation
WHERE a.attnum > 0
  AND NOT a.attisdropped
  AND c.relkind = 'r'
  AND n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
  AND n.nspname <> 'information_schema'
ORDER BY n.nspname, c.relname, a.attnum;
