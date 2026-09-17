-- name: CreateAuditRun :one
INSERT INTO audit_run (
  environment_id, profile, status, service_version, collector_version, warnings, errors
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, environment_id, profile, status, service_version, collector_version,
  started_at, finished_at, warnings, errors;

-- name: GetAuditRunByID :one
SELECT id, environment_id, profile, status, service_version, collector_version,
  started_at, finished_at, warnings, errors
FROM audit_run
WHERE id = $1;

-- name: FinishAuditRun :one
UPDATE audit_run
SET status = $2, finished_at = now(), warnings = $3, errors = $4
WHERE id = $1
RETURNING id, environment_id, profile, status, service_version, collector_version,
  started_at, finished_at, warnings, errors;

-- name: CreateCollectorRun :one
INSERT INTO collector_run (
  audit_run_id, collector_name, collector_version, status, query_name
) VALUES ($1, $2, $3, $4, $5)
RETURNING id, audit_run_id, collector_name, collector_version, status, query_name,
  started_at, finished_at, rows_collected, warning, error;

-- name: FinishCollectorRun :one
UPDATE collector_run
SET status = $2, finished_at = now(), rows_collected = $3, warning = $4, error = $5
WHERE id = $1
RETURNING id, audit_run_id, collector_name, collector_version, status, query_name,
  started_at, finished_at, rows_collected, warning, error;

-- name: ListCollectorRunsByAuditRun :many
SELECT id, audit_run_id, collector_name, collector_version, status, query_name,
  started_at, finished_at, rows_collected, warning, error
FROM collector_run
WHERE audit_run_id = $1
ORDER BY started_at;
