package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mayconmendes-qc/db-auditor/internal/analyzer"
)

type AnalysisRun struct {
	ID               string     `json:"id"`
	AuditRunID       string     `json:"audit_run_id"`
	EnvironmentID    string     `json:"environment_id"`
	Status           string     `json:"status"`
	AnalyzerVersion  string     `json:"analyzer_version"`
	FindingsProduced int        `json:"findings_produced"`
	FindingsSaved    int        `json:"findings_saved"`
	Error            *string    `json:"error,omitempty"`
	StartedAt        time.Time  `json:"started_at"`
	FinishedAt       *time.Time `json:"finished_at,omitempty"`
}

func (s *Store) StartAnalysisRun(ctx context.Context, environmentID, auditRunID, version string) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO analysis_run (audit_run_id, environment_id, status, analyzer_version)
VALUES ($1::uuid, $2::uuid, 'running', $3)
ON CONFLICT (audit_run_id) DO UPDATE SET
  status = 'running', analyzer_version = EXCLUDED.analyzer_version,
  findings_produced = 0, findings_saved = 0, error = NULL,
  started_at = now(), finished_at = NULL
`, auditRunID, environmentID, version)
	return err
}

func (s *Store) FinishAnalysisRun(ctx context.Context, auditRunID, status string, produced, saved int, errMsg string) error {
	_, err := s.pool.Exec(ctx, `
UPDATE analysis_run SET status=$2, findings_produced=$3, findings_saved=$4,
  error=NULLIF($5,''), finished_at=now()
WHERE audit_run_id=$1::uuid
`, auditRunID, status, produced, saved, errMsg)
	return err
}

func (s *Store) GetAnalysisRun(ctx context.Context, auditRunID string) (*AnalysisRun, error) {
	var r AnalysisRun
	err := s.pool.QueryRow(ctx, `
SELECT id::text, audit_run_id::text, environment_id::text, status, analyzer_version,
  findings_produced, findings_saved, error, started_at, finished_at
FROM analysis_run WHERE audit_run_id=$1::uuid
`, auditRunID).Scan(&r.ID, &r.AuditRunID, &r.EnvironmentID, &r.Status, &r.AnalyzerVersion,
		&r.FindingsProduced, &r.FindingsSaved, &r.Error, &r.StartedAt, &r.FinishedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Store) SaveAnalysisFindings(ctx context.Context, findings []analyzer.Finding) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for i, f := range findings {
		refs, _ := json.Marshal(f.References)
		params, _ := json.Marshal(f.RuleParameters)
		cmd, err := tx.Exec(ctx, upsertFindingSQL,
			f.EnvironmentID, f.AuditRunID, f.FindingType, string(f.Severity),
			f.Title, f.Summary, f.ObjectType, f.ObjectKey,
			f.DatabaseName, f.SchemaName, f.ObjectName, analyzer.EvidenceJSON(f.Evidence), f.DedupKey,
			f.RuleID, f.RuleVersion, f.Category, f.Confidence, f.Impact, f.Risk, f.Recommendation, f.Validation, refs, params)
		if err != nil {
			return 0, fmt.Errorf("finding %d (%s): %w", i+1, f.FindingType, err)
		}
		if cmd.RowsAffected() != 1 {
			return 0, fmt.Errorf("finding %d (%s) was not persisted", i+1, f.FindingType)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return len(findings), nil
}
