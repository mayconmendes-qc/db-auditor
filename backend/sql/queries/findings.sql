-- Findings queue. Empty string means "no filter". Status open includes an
-- expired suppression; status suppressed is only a still-valid suppression.

-- name: CountFindingQueue :one
SELECT count(*)::bigint
FROM finding
WHERE ($1 = '' OR environment_id = $1::uuid)
  AND ($2 = '' OR finding_type = $2)
  AND ($3 = '' OR severity = $3)
  AND (
    $4 = ''
    OR ($4 = 'open' AND (status = 'open' OR (status = 'suppressed' AND suppressed_until <= now())))
    OR ($4 = 'suppressed' AND status = 'suppressed' AND (suppressed_until IS NULL OR suppressed_until > now()))
    OR ($4 NOT IN ('', 'open', 'suppressed') AND status = $4)
  )
  AND ($5 = '' OR assignee = $5)
  AND ($6::boolean = false OR (due_at IS NOT NULL AND due_at < now() AND status IN ('open', 'acknowledged')));

-- name: ListFindingQueue :many
SELECT id::text, environment_id::text, audit_run_id::text,
  finding_type, severity, status, title, summary,
  object_type, object_key, database_name, schema_name, object_name,
  evidence, dedup_key, rule_id, rule_version, category, confidence, impact, risk,
  recommendation, validation, reference_urls, rule_parameters, first_seen_at, last_seen_at, resolved_at, notes,
  created_at, updated_at, recurrence_count, suppression_reason, suppressed_until, superseded_by::text, assignee, due_at
FROM finding
WHERE ($1 = '' OR environment_id = $1::uuid)
  AND ($2 = '' OR finding_type = $2)
  AND ($3 = '' OR severity = $3)
  AND (
    $4 = ''
    OR ($4 = 'open' AND (status = 'open' OR (status = 'suppressed' AND suppressed_until <= now())))
    OR ($4 = 'suppressed' AND status = 'suppressed' AND (suppressed_until IS NULL OR suppressed_until > now()))
    OR ($4 NOT IN ('', 'open', 'suppressed') AND status = $4)
  )
  AND ($5 = '' OR assignee = $5)
  AND ($6::boolean = false OR (due_at IS NOT NULL AND due_at < now() AND status IN ('open', 'acknowledged')))
ORDER BY last_seen_at DESC, id DESC
LIMIT $7 OFFSET $8;
