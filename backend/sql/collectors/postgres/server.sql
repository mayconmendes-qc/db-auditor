-- Server facts only (read-only). Used by internal/collectors/postgres.
SELECT
  current_setting('server_version') AS server_version,
  pg_postmaster_start_time() AS started_at,
  now() - pg_postmaster_start_time() AS uptime,
  current_setting('TimeZone') AS timezone,
  current_setting('server_encoding') AS server_encoding,
  current_setting('max_connections')::int AS max_connections,
  (SELECT count(*) FROM pg_stat_activity) AS current_connections,
  current_setting('autovacuum', true) AS autovacuum
;
