package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// MigrationAnalysis is the header row for a submitted CLI analysis.
type MigrationAnalysis struct {
	ID              string    `json:"id"`
	EnvironmentID   string    `json:"environment_id"`
	Repository      string    `json:"repository"`
	PR              string    `json:"pr"`
	CommitSHA       string    `json:"commit"`
	HighestSeverity string    `json:"highest_severity"`
	RulesVersion    string    `json:"rules_version"`
	PGVersion       int       `json:"pg_version"`
	ContextMode     string    `json:"context_mode"`
	AuditRunID      *string   `json:"audit_run_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// MigrationAnalysisItem is one finding row (no SQL text).
type MigrationAnalysisItem struct {
	FilePath         string `json:"file"`
	LineNo           int    `json:"line"`
	Code             string `json:"code"`
	Operation        string `json:"operation"`
	ObjectName       string `json:"object"`
	LockMode         string `json:"lock"`
	RewriteOrScan    string `json:"rewrite_or_scan"`
	SizeBytes        *int64 `json:"size_bytes,omitempty"`
	DurationEstimate string `json:"duration_estimate,omitempty"`
	Severity         string `json:"severity"`
	Confidence       string `json:"confidence"`
	Suppressed       bool   `json:"suppressed"`
	Justification    string `json:"justification,omitempty"`
}

// CreateMigrationAnalysisParams is the validated write payload.
type CreateMigrationAnalysisParams struct {
	EnvironmentID   string
	Repository      string
	PR              string
	CommitSHA       string
	HighestSeverity string
	RulesVersion    string
	PGVersion       int
	ContextMode     string
	AuditRunID      string
	TokenID         string
	Items           []MigrationAnalysisItem
}

var allowedSeverities = map[string]bool{
	"info": true, "low": true, "medium": true, "high": true, "critical": true,
}

// CreateMigrationAnalysis persists a CLI result without SQL/literals.
func (s *Store) CreateMigrationAnalysis(ctx context.Context, p CreateMigrationAnalysisParams) (*MigrationAnalysis, error) {
	if !allowedSeverities[p.HighestSeverity] {
		return nil, errors.New("highest_severity inválido")
	}
	if p.PGVersion < 14 || p.PGVersion > 18 {
		return nil, errors.New("pg_version inválido")
	}
	switch p.ContextMode {
	case "none", "snapshot", "direct":
	default:
		return nil, errors.New("context_mode inválido")
	}
	p.Repository = clip(p.Repository, 200)
	p.PR = clip(p.PR, 32)
	p.CommitSHA = clip(p.CommitSHA, 64)
	p.RulesVersion = clip(p.RulesVersion, 32)
	if len(p.Items) > 500 {
		return nil, errors.New("itens demais")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var id string
	var created time.Time
	var audit any
	if p.AuditRunID != "" {
		audit = p.AuditRunID
	}
	var token any
	if p.TokenID != "" {
		token = p.TokenID
	}
	err = tx.QueryRow(ctx, `
INSERT INTO migration_analysis(environment_id,repository,pr,commit_sha,highest_severity,rules_version,pg_version,context_mode,audit_run_id,created_by_token)
VALUES($1::uuid,$2,$3,$4,$5,$6,$7,$8,$9::uuid,$10::uuid)
RETURNING id::text, created_at`,
		p.EnvironmentID, p.Repository, p.PR, p.CommitSHA, p.HighestSeverity, p.RulesVersion, p.PGVersion, p.ContextMode, audit, token,
	).Scan(&id, &created)
	if err != nil {
		return nil, err
	}
	for _, it := range p.Items {
		if !allowedSeverities[it.Severity] {
			return nil, errors.New("severity de item inválido")
		}
		_, err = tx.Exec(ctx, `
INSERT INTO migration_analysis_item(analysis_id,file_path,line_no,code,operation,object_name,lock_mode,rewrite_or_scan,size_bytes,duration_estimate,severity,confidence,suppressed,justification)
VALUES($1::uuid,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
			id, clip(it.FilePath, 512), it.LineNo, clip(it.Code, 16), clip(it.Operation, 64),
			clip(it.ObjectName, 256), clip(it.LockMode, 64), clip(it.RewriteOrScan, 128),
			it.SizeBytes, clip(it.DurationEstimate, 64), it.Severity, clip(it.Confidence, 16),
			it.Suppressed, clip(it.Justification, 500))
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	out := &MigrationAnalysis{
		ID: id, EnvironmentID: p.EnvironmentID, Repository: p.Repository, PR: p.PR,
		CommitSHA: p.CommitSHA, HighestSeverity: p.HighestSeverity, RulesVersion: p.RulesVersion,
		PGVersion: p.PGVersion, ContextMode: p.ContextMode, CreatedAt: created.UTC(),
	}
	if p.AuditRunID != "" {
		out.AuditRunID = &p.AuditRunID
	}
	return out, nil
}

// ListMigrationAnalyses returns recent analyses for an environment.
func (s *Store) ListMigrationAnalyses(ctx context.Context, environmentID string, limit int) ([]MigrationAnalysis, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
SELECT id::text, environment_id::text, repository, pr, commit_sha, highest_severity, rules_version, pg_version, context_mode, audit_run_id::text, created_at
FROM migration_analysis WHERE environment_id=$1::uuid
ORDER BY created_at DESC LIMIT $2`, environmentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]MigrationAnalysis, 0)
	for rows.Next() {
		var a MigrationAnalysis
		var audit *string
		if err := rows.Scan(&a.ID, &a.EnvironmentID, &a.Repository, &a.PR, &a.CommitSHA, &a.HighestSeverity,
			&a.RulesVersion, &a.PGVersion, &a.ContextMode, &audit, &a.CreatedAt); err != nil {
			return nil, err
		}
		a.AuditRunID = audit
		a.CreatedAt = a.CreatedAt.UTC()
		items = append(items, a)
	}
	return items, rows.Err()
}

// GetMigrationAnalysis returns header + items.
func (s *Store) GetMigrationAnalysis(ctx context.Context, environmentID, analysisID string) (*MigrationAnalysis, []MigrationAnalysisItem, error) {
	var a MigrationAnalysis
	var audit *string
	err := s.pool.QueryRow(ctx, `
SELECT id::text, environment_id::text, repository, pr, commit_sha, highest_severity, rules_version, pg_version, context_mode, audit_run_id::text, created_at
FROM migration_analysis WHERE id=$1::uuid AND environment_id=$2::uuid`, analysisID, environmentID).Scan(
		&a.ID, &a.EnvironmentID, &a.Repository, &a.PR, &a.CommitSHA, &a.HighestSeverity,
		&a.RulesVersion, &a.PGVersion, &a.ContextMode, &audit, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	a.AuditRunID = audit
	a.CreatedAt = a.CreatedAt.UTC()
	rows, err := s.pool.Query(ctx, `
SELECT file_path, line_no, code, operation, object_name, lock_mode, rewrite_or_scan, size_bytes, duration_estimate, severity, confidence, suppressed, justification
FROM migration_analysis_item WHERE analysis_id=$1::uuid ORDER BY id`, analysisID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	items := make([]MigrationAnalysisItem, 0)
	for rows.Next() {
		var it MigrationAnalysisItem
		if err := rows.Scan(&it.FilePath, &it.LineNo, &it.Code, &it.Operation, &it.ObjectName, &it.LockMode,
			&it.RewriteOrScan, &it.SizeBytes, &it.DurationEstimate, &it.Severity, &it.Confidence, &it.Suppressed, &it.Justification); err != nil {
			return nil, nil, err
		}
		items = append(items, it)
	}
	return &a, items, rows.Err()
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n]
	}
	return s
}
