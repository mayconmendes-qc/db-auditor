package repository

import (
	"context"
	"time"
)

type AuditRunCoverage struct {
	CollectorName string    `json:"collector_name"`
	DatabaseName  string    `json:"database_name,omitempty"`
	Status        string    `json:"status"`
	RowsCollected int64     `json:"rows_collected"`
	Warning       *string   `json:"warning,omitempty"`
	Error         *string   `json:"error,omitempty"`
	CollectedAt   time.Time `json:"collected_at"`
}

func (s *Store) ListAuditRunCoverage(ctx context.Context, auditRunID string) ([]AuditRunCoverage, error) {
	rows, err := s.pool.Query(ctx, `
SELECT collector_name,database_name,status,rows_collected,warning,error,collected_at
FROM audit_run_coverage WHERE audit_run_id=$1::uuid ORDER BY collector_name,database_name
`, auditRunID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]AuditRunCoverage, 0)
	for rows.Next() {
		var x AuditRunCoverage
		if err := rows.Scan(&x.CollectorName, &x.DatabaseName, &x.Status, &x.RowsCollected, &x.Warning, &x.Error, &x.CollectedAt); err != nil {
			return nil, err
		}
		items = append(items, x)
	}
	return items, rows.Err()
}
