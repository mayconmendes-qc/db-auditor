package repository

import (
	"context"
	"fmt"
)

// ListFindingsCategoryPage keeps specialized screens server-paginated without
// silently truncating findings after a fixed client-side limit.
func (s *Store) ListFindingsCategoryPage(ctx context.Context, environmentID, category, status string, limit, offset int) ([]Finding, int, error) {
	if (category != "security" && category != "performance") || limit < 1 || limit > 500 || offset < 0 {
		return nil, 0, fmt.Errorf("invalid category pagination")
	}
	where := `WHERE ($1='' OR environment_id=$1::uuid) AND ($2='' OR status=$2)
		AND (($3='security' AND finding_type LIKE 'security.%') OR
		($3='performance' AND (finding_type LIKE 'performance.%' OR finding_type LIKE 'vacuum.%')))`
	var total int
	if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM finding "+where, environmentID, status, category).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id::text, environment_id::text, audit_run_id::text,
  finding_type, severity, status, title, summary,
  object_type, object_key, database_name, schema_name, object_name,
  evidence, dedup_key, rule_id, rule_version, category, confidence, impact, risk,
  recommendation, validation, reference_urls, rule_parameters, first_seen_at, last_seen_at, resolved_at, notes,
  created_at, updated_at, recurrence_count, suppression_reason, suppressed_until, superseded_by::text, assignee, due_at
FROM finding `+where+` ORDER BY last_seen_at DESC,id DESC LIMIT $4 OFFSET $5`, environmentID, status, category, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]Finding, 0)
	for rows.Next() {
		item, err := scanFinding(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *item)
	}
	return items, total, rows.Err()
}
