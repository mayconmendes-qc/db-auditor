package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// InventoryCounts is always scoped to one terminal audit run.
type InventoryCounts struct {
	AuditRunID string `json:"audit_run_id"`
	Status     string `json:"status"`
	Databases  int    `json:"databases"`
	Schemas    int    `json:"schemas"`
	Tables     int    `json:"tables"`
}

func (s *Store) CountLatestInventory(ctx context.Context, environmentID string) (*InventoryCounts, error) {
	var counts InventoryCounts
	err := s.pool.QueryRow(ctx, `WITH latest AS (
  SELECT id, status FROM audit_run
  WHERE environment_id=$1::uuid AND status IN ('success','partial_success')
  ORDER BY started_at DESC,id DESC LIMIT 1
)
SELECT latest.id::text,latest.status,
  (SELECT count(*) FROM database_snapshot d WHERE d.audit_run_id=latest.id AND NOT d.is_template),
  (SELECT count(*) FROM schema_snapshot sc WHERE sc.audit_run_id=latest.id),
  (SELECT count(*) FROM table_snapshot t WHERE t.audit_run_id=latest.id)
FROM latest`, environmentID).Scan(&counts.AuditRunID, &counts.Status, &counts.Databases, &counts.Schemas, &counts.Tables)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	completeness, err := s.GetSnapshotCompleteness(ctx, environmentID, counts.AuditRunID)
	if err != nil {
		return nil, err
	}
	counts.Status = completeness.Completeness
	return &counts, nil
}
