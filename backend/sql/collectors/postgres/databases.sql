-- Database inventory facts (read-only).
SELECT
  d.datname AS database_name,
  pg_catalog.pg_get_userbyid(d.datdba) AS owner_name,
  pg_catalog.pg_encoding_to_char(d.encoding) AS encoding,
  d.datcollate AS collate_name,
  d.datctype AS ctype_name,
  d.datallowconn AS allow_connections,
  d.datistemplate AS is_template,
  COALESCE(pg_database_size(d.datname), 0) AS size_bytes,
  COALESCE(s.numbackends, 0) AS connection_count
FROM pg_database d
LEFT JOIN pg_stat_database s ON s.datname = d.datname
ORDER BY d.datname;
