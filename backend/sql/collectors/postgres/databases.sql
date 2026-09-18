-- postgres.databases — list databases with scope-friendly metadata.
-- size only when CONNECT is allowed (avoids 42501 on Tiger Cloud system DBs).
SELECT
  d.datname AS database_name,
  pg_catalog.pg_get_userbyid(d.datdba) AS owner_name,
  pg_catalog.pg_encoding_to_char(d.encoding) AS encoding,
  d.datcollate AS collate_name,
  d.datctype AS ctype_name,
  d.datallowconn AS allow_connections,
  d.datistemplate AS is_template,
  CASE
    WHEN has_database_privilege(d.datname, 'CONNECT')
    THEN COALESCE(pg_database_size(d.datname), 0)
    ELSE 0
  END AS size_bytes,
  COALESCE(s.numbackends, 0) AS connection_count
FROM pg_database d
LEFT JOIN pg_stat_database s ON s.datname = d.datname
ORDER BY d.datname;
