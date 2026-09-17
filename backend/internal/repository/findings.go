package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// Finding is a persisted diagnostic.
type Finding struct {
	ID            string          `json:"id"`
	EnvironmentID string          `json:"environment_id"`
	AuditRunID    *string         `json:"audit_run_id,omitempty"`
	FindingType   string          `json:"finding_type"`
	Severity      string          `json:"severity"`
	Status        string          `json:"status"`
	Title         string          `json:"title"`
	Summary       string          `json:"summary"`
	ObjectType    string          `json:"object_type"`
	ObjectKey     string          `json:"object_key"`
	DatabaseName  string          `json:"database_name"`
	SchemaName    string          `json:"schema_name"`
	ObjectName    string          `json:"object_name"`
	Evidence      json.RawMessage `json:"evidence"`
	DedupKey      string          `json:"dedup_key"`
	FirstSeenAt   time.Time       `json:"first_seen_at"`
	LastSeenAt    time.Time       `json:"last_seen_at"`
	ResolvedAt    *time.Time      `json:"resolved_at,omitempty"`
	Notes         *string         `json:"notes,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// UpsertFindingParams is the input for inserting or refreshing a finding.
type UpsertFindingParams struct {
	EnvironmentID string
	AuditRunID    string
	FindingType   string
	Severity      string
	Title         string
	Summary       string
	ObjectType    string
	ObjectKey     string
	DatabaseName  string
	SchemaName    string
	ObjectName    string
	Evidence      []byte
	DedupKey      string
}

// UpsertFinding inserts a new open finding or refreshes last_seen on match.
func (s *Store) UpsertFinding(ctx context.Context, p UpsertFindingParams) (*Finding, error) {
	if len(p.Evidence) == 0 {
		p.Evidence = []byte("{}")
	}
	row := s.pool.QueryRow(ctx, `
INSERT INTO finding (
  environment_id, audit_run_id, finding_type, severity, status,
  title, summary, object_type, object_key,
  database_name, schema_name, object_name,
  evidence, dedup_key, first_seen_at, last_seen_at
) VALUES (
  $1::uuid, NULLIF($2,'')::uuid, $3, $4, 'open',
  $5, $6, $7, $8,
  $9, $10, $11,
  $12::jsonb, $13, now(), now()
)
ON CONFLICT (environment_id, dedup_key) DO UPDATE SET
  last_seen_at = now(),
  audit_run_id = COALESCE(EXCLUDED.audit_run_id, finding.audit_run_id),
  severity = EXCLUDED.severity,
  title = EXCLUDED.title,
  summary = EXCLUDED.summary,
  evidence = EXCLUDED.evidence,
  updated_at = now()
WHERE finding.status IN ('open', 'acknowledged')
RETURNING id::text, environment_id::text, audit_run_id::text,
  finding_type, severity, status, title, summary,
  object_type, object_key, database_name, schema_name, object_name,
  evidence, dedup_key, first_seen_at, last_seen_at, resolved_at, notes,
  created_at, updated_at
`, p.EnvironmentID, p.AuditRunID, p.FindingType, p.Severity,
		p.Title, p.Summary, p.ObjectType, p.ObjectKey,
		p.DatabaseName, p.SchemaName, p.ObjectName,
		p.Evidence, p.DedupKey,
	)
	return scanFinding(row)
}

// ListFindings filters findings by environment and optional dimensions.
func (s *Store) ListFindings(ctx context.Context, environmentID, findingType, severity, status string, limit int) ([]Finding, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
SELECT id::text, environment_id::text, audit_run_id::text,
  finding_type, severity, status, title, summary,
  object_type, object_key, database_name, schema_name, object_name,
  evidence, dedup_key, first_seen_at, last_seen_at, resolved_at, notes,
  created_at, updated_at
FROM finding
WHERE ($1 = '' OR environment_id = $1::uuid)
  AND ($2 = '' OR finding_type = $2)
  AND ($3 = '' OR severity = $3)
  AND ($4 = '' OR status = $4)
ORDER BY last_seen_at DESC
LIMIT $5
`, environmentID, findingType, severity, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Finding, 0)
	for rows.Next() {
		f, err := scanFinding(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *f)
	}
	return out, rows.Err()
}

// GetFinding returns one finding by id.
func (s *Store) GetFinding(ctx context.Context, id string) (*Finding, error) {
	row := s.pool.QueryRow(ctx, `
SELECT id::text, environment_id::text, audit_run_id::text,
  finding_type, severity, status, title, summary,
  object_type, object_key, database_name, schema_name, object_name,
  evidence, dedup_key, first_seen_at, last_seen_at, resolved_at, notes,
  created_at, updated_at
FROM finding WHERE id = $1::uuid
`, id)
	return scanFinding(row)
}

// UpdateFindingStatus changes triage status and optional notes.
func (s *Store) UpdateFindingStatus(ctx context.Context, id, status, notes string) (*Finding, error) {
	row := s.pool.QueryRow(ctx, `
UPDATE finding
SET status = $2,
    notes = COALESCE(NULLIF($3,''), notes),
    resolved_at = CASE WHEN $2 IN ('resolved', 'suppressed') THEN now() ELSE resolved_at END,
    updated_at = now()
WHERE id = $1::uuid
RETURNING id::text, environment_id::text, audit_run_id::text,
  finding_type, severity, status, title, summary,
  object_type, object_key, database_name, schema_name, object_name,
  evidence, dedup_key, first_seen_at, last_seen_at, resolved_at, notes,
  created_at, updated_at
`, id, status, notes)
	f, err := scanFinding(row)
	if err != nil {
		return nil, fmt.Errorf("update finding: %w", err)
	}
	return f, nil
}

func scanFinding(row scannable) (*Finding, error) {
	var f Finding
	var auditRun, notes pgtype.Text
	var resolvedAt pgtype.Timestamptz
	var evidence []byte
	if err := row.Scan(
		&f.ID, &f.EnvironmentID, &auditRun,
		&f.FindingType, &f.Severity, &f.Status, &f.Title, &f.Summary,
		&f.ObjectType, &f.ObjectKey, &f.DatabaseName, &f.SchemaName, &f.ObjectName,
		&evidence, &f.DedupKey, &f.FirstSeenAt, &f.LastSeenAt, &resolvedAt, &notes,
		&f.CreatedAt, &f.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if auditRun.Valid {
		s := auditRun.String
		f.AuditRunID = &s
	}
	if notes.Valid {
		s := notes.String
		f.Notes = &s
	}
	if resolvedAt.Valid {
		t := resolvedAt.Time
		f.ResolvedAt = &t
	}
	if len(evidence) == 0 {
		f.Evidence = json.RawMessage("{}")
	} else {
		f.Evidence = evidence
	}
	return &f, nil
}
