package postgres

// SQL kept in sync with backend/sql/collectors/postgres/*.sql (source of truth for review).
// When changing a query: update the .sql file under backend/sql/collectors/ first, then mirror here.
// Runtime collectors execute the constants in this file — the .sql files are documentation/review only.

const serverSQL = `
SELECT
  current_setting('server_version') AS server_version,
  pg_postmaster_start_time() AS started_at,
  (now() - pg_postmaster_start_time())::text AS uptime,
  current_setting('TimeZone') AS timezone,
  current_setting('server_encoding') AS server_encoding,
  current_setting('max_connections')::int AS max_connections,
  (SELECT count(*) FROM pg_stat_activity) AS current_connections,
  current_setting('autovacuum', true) AS autovacuum
`
