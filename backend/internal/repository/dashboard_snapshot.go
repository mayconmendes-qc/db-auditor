package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// LatestEnvironmentSnapshot reads one terminal run and every dashboard
// inventory counter in a single PostgreSQL statement. A newly finished run
// cannot be mixed with counters from the previous run.
type LatestEnvironmentSnapshot struct {
	Inventory    InventoryCounts
	Capabilities CapabilityAggregate
}

func (s *Store) CountLatestEnvironmentSnapshot(ctx context.Context, environmentID string) (*LatestEnvironmentSnapshot, error) {
	var out LatestEnvironmentSnapshot
	err := s.pool.QueryRow(ctx, `WITH latest AS (
 SELECT id,status FROM audit_run WHERE environment_id=$1::uuid
 AND status IN ('success','partial_success') ORDER BY started_at DESC,id DESC LIMIT 1
)
SELECT latest.id::text, CASE WHEN latest.status='success' AND NOT EXISTS (
 SELECT 1 FROM audit_run_coverage c WHERE c.audit_run_id=latest.id AND c.status='failed'
) THEN 'complete' ELSE 'partial' END,
 (SELECT count(*) FROM database_snapshot d WHERE d.audit_run_id=latest.id AND NOT d.is_template),
 (SELECT count(*) FROM schema_snapshot sc WHERE sc.audit_run_id=latest.id),
 (SELECT count(*) FROM table_snapshot t WHERE t.audit_run_id=latest.id),
 COALESCE((SELECT sum(size_bytes) FROM database_snapshot d WHERE d.audit_run_id=latest.id AND NOT d.is_template),0),
 (SELECT count(*) FROM hypertable_snapshot h WHERE h.audit_run_id=latest.id),
 (SELECT count(*) FROM job_snapshot j WHERE j.audit_run_id=latest.id AND j.scheduled),
 (SELECT count(*) FROM policy_snapshot p WHERE p.audit_run_id=latest.id)
FROM latest`, environmentID).Scan(&out.Inventory.AuditRunID, &out.Inventory.Status,
		&out.Inventory.Databases, &out.Inventory.Schemas, &out.Inventory.Tables,
		&out.Capabilities.TotalStorageBytes, &out.Capabilities.Hypertables,
		&out.Capabilities.JobsScheduled, &out.Capabilities.Policies)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}
