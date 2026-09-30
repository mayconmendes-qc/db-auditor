package repository

import (
	"context"
	"fmt"
)

// ResolveAuditRunID returns a requested run after validating its environment,
// or the latest terminal run for that environment. All snapshot families use
// this same resolution so a response never silently mixes audit runs.
func (s *Store) ResolveAuditRunID(ctx context.Context, environmentID, requested string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
SELECT id::text
FROM audit_run
WHERE environment_id = $1::uuid
  AND ($2 = '' OR id = $2::uuid)
  AND status IN ('success', 'partial_success')
ORDER BY started_at DESC
LIMIT 1
`, environmentID, requested).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("resolve audit run for environment %s: %w", environmentID, err)
	}
	return id, nil
}
