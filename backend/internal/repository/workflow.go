package repository

import (
	"context"
	"time"
)

func (s *Store) UpdateFindingWorkflow(ctx context.Context, id, assignee string, due *time.Time) (*Finding, error) {
	row := s.pool.QueryRow(ctx, `UPDATE finding SET assignee=$2, due_at=$3, updated_at=now() WHERE id=$1::uuid
RETURNING id::text, environment_id::text, audit_run_id::text,
  finding_type, severity, status, title, summary,
  object_type, object_key, database_name, schema_name, object_name,
  evidence, dedup_key, rule_id, rule_version, category, confidence, impact, risk,
  recommendation, validation, reference_urls, rule_parameters, first_seen_at, last_seen_at, resolved_at, notes,
  created_at, updated_at, recurrence_count, suppression_reason, suppressed_until, superseded_by::text, assignee, due_at`, id, assignee, due)
	return scanFinding(row)
}

func (s *Store) ListRunFindings(ctx context.Context, auditRunID string) ([]Finding, error) {
	rows, err := s.pool.Query(ctx, `SELECT f.id::text, f.environment_id::text, e.audit_run_id::text,
  f.finding_type, e.severity, e.finding_status, e.title, e.summary,
  f.object_type, f.object_key, e.database_name, e.schema_name, e.object_name,
  e.evidence, f.dedup_key, f.rule_id, e.rule_version, e.category, e.confidence, e.impact, e.risk,
  e.recommendation, e.validation, e.reference_urls, e.rule_parameters, f.first_seen_at, f.last_seen_at, f.resolved_at, f.notes,
  f.created_at, e.recorded_at, f.recurrence_count, f.suppression_reason, f.suppressed_until, f.superseded_by::text, f.assignee, f.due_at
FROM finding_event e
JOIN finding f ON f.id = e.finding_id
WHERE e.audit_run_id=$1::uuid AND e.event_type='observed'
ORDER BY e.recorded_at, e.id`, auditRunID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Finding
	for rows.Next() {
		item, err := scanFinding(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}
