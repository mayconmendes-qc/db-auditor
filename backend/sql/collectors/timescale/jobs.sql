-- TimescaleDB jobs inventory (read-only).
SELECT
  current_database() AS database_name,
  j.job_id::bigint AS job_id,
  j.application_name AS application_name,
  j.schedule_interval::text AS schedule_interval,
  j.max_runtime::text AS max_runtime,
  j.max_retries::int AS max_retries,
  j.retry_period::text AS retry_period,
  j.proc_schema AS proc_schema,
  j.proc_name AS proc_name,
  j.owner AS owner_name,
  COALESCE(j.scheduled, false) AS scheduled,
  COALESCE(j.fixed_schedule, false) AS fixed_schedule,
  j.config::text AS config_json,
  j.next_start AS next_start,
  j.initial_start AS initial_start,
  j.hypertable_schema AS hypertable_schema,
  j.hypertable_name AS hypertable_name,
  j.check_schema AS check_schema,
  j.check_name AS check_name
FROM timescaledb_information.jobs j
ORDER BY j.job_id;
