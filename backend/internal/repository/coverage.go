package repository

import (
	"context"
	"strings"
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

// SnapshotCompleteness summarizes whether the latest coherent run is complete.
type SnapshotCompleteness struct {
	AuditRunID       string  `json:"audit_run_id"`
	RunStatus        string  `json:"run_status"`
	Completeness     string  `json:"completeness"` // complete | partial | empty | unknown
	FailedDatabases  int     `json:"failed_databases"`
	FailedCollectors int     `json:"failed_collectors"`
	CoverageRows     int     `json:"coverage_rows"`
	AnalysisStatus   *string `json:"analysis_status,omitempty"`
	AnalysisProduced int     `json:"analysis_findings_produced"`
	AnalysisSaved    int     `json:"analysis_findings_saved"`
	StartedAt        string  `json:"started_at,omitempty"`
	FinishedAt       *string `json:"finished_at,omitempty"`
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

// GetSnapshotCompleteness resolves the latest terminal run for the environment
// and derives a completeness label from audit_run + coverage + analysis_run.
func (s *Store) GetSnapshotCompleteness(ctx context.Context, environmentID, requestedRunID string) (*SnapshotCompleteness, error) {
	runID, err := s.ResolveAuditRunID(ctx, environmentID, requestedRunID)
	if err != nil {
		return &SnapshotCompleteness{Completeness: "empty"}, nil
	}
	var runStatus string
	var startedAt time.Time
	var finishedAt *time.Time
	if err := s.pool.QueryRow(ctx, `
SELECT status, started_at, finished_at FROM audit_run WHERE id=$1::uuid
`, runID).Scan(&runStatus, &startedAt, &finishedAt); err != nil {
		return nil, err
	}
	coverage, err := s.ListAuditRunCoverage(ctx, runID)
	if err != nil {
		return nil, err
	}
	failedDB := map[string]struct{}{}
	failedCollectors := 0
	for _, c := range coverage {
		st := strings.ToLower(c.Status)
		if st == "failed" {
			if c.DatabaseName != "" {
				failedDB[c.DatabaseName] = struct{}{}
			} else {
				failedCollectors++
			}
		}
	}
	out := &SnapshotCompleteness{
		AuditRunID:       runID,
		RunStatus:        runStatus,
		FailedDatabases:  len(failedDB),
		FailedCollectors: failedCollectors,
		CoverageRows:     len(coverage),
		StartedAt:        startedAt.UTC().Format(time.RFC3339),
	}
	if finishedAt != nil {
		s := finishedAt.UTC().Format(time.RFC3339)
		out.FinishedAt = &s
	}
	if ar, err := s.GetAnalysisRun(ctx, runID); err == nil && ar != nil {
		st := ar.Status
		out.AnalysisStatus = &st
		out.AnalysisProduced = ar.FindingsProduced
		out.AnalysisSaved = ar.FindingsSaved
	}
	switch {
	case strings.EqualFold(runStatus, "partial_success") || len(failedDB) > 0 || failedCollectors > 0:
		out.Completeness = "partial"
	case strings.EqualFold(runStatus, "success"):
		out.Completeness = "complete"
	default:
		out.Completeness = "unknown"
	}
	return out, nil
}
