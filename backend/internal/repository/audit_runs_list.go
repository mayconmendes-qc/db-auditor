package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type AuditRunRow struct {
	ID               string     `json:"id"`
	EnvironmentID    string     `json:"environment_id"`
	EnvironmentName  string     `json:"environment_name"`
	Profile          string     `json:"profile"`
	Status           string     `json:"status"`
	ServiceVersion   string     `json:"service_version"`
	CollectorVersion string     `json:"collector_version"`
	StartedAt        time.Time  `json:"started_at"`
	FinishedAt       *time.Time `json:"finished_at,omitempty"`
	Warnings         []string   `json:"warnings"`
	Errors           []string   `json:"errors"`
}

type CollectorRunRow struct {
	ID               string     `json:"id"`
	AuditRunID       string     `json:"audit_run_id"`
	CollectorName    string     `json:"collector_name"`
	CollectorVersion string     `json:"collector_version"`
	Status           string     `json:"status"`
	StartedAt        time.Time  `json:"started_at"`
	FinishedAt       *time.Time `json:"finished_at,omitempty"`
	RowsCollected    int64      `json:"rows_collected"`
	Warning          *string    `json:"warning,omitempty"`
	Error            *string    `json:"error,omitempty"`
}

// ListAuditRuns returns recent runs with environment name, optionally filtered.
func (s *Store) ListAuditRuns(ctx context.Context, environmentID, profile, status string, limit int) ([]AuditRunRow, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.listAuditRuns(ctx, environmentID, profile, status, limit, 0)
}

func (s *Store) ListAuditRunsPage(ctx context.Context, environmentID, profile, status string, limit, offset int) ([]AuditRunRow, int, error) {
	if limit < 1 || limit > 500 || offset < 0 {
		return nil, 0, fmt.Errorf("invalid pagination")
	}
	var total int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM audit_run WHERE ($1='' OR environment_id=$1::uuid) AND ($2='' OR profile=$2) AND ($3='' OR status=$3)`, environmentID, profile, status).Scan(&total)
	if err != nil {
		return nil, 0, err
	}
	items, err := s.listAuditRuns(ctx, environmentID, profile, status, limit, offset)
	return items, total, err
}

func (s *Store) listAuditRuns(ctx context.Context, environmentID, profile, status string, limit, offset int) ([]AuditRunRow, error) {
	rows, err := s.pool.Query(ctx, `
SELECT ar.id::text, ar.environment_id::text, COALESCE(e.name, ''), ar.profile, ar.status,
  ar.service_version, ar.collector_version, ar.started_at, ar.finished_at, ar.warnings, ar.errors
FROM audit_run ar
LEFT JOIN audit_environment e ON e.id = ar.environment_id
WHERE ($1 = '' OR ar.environment_id = $1::uuid)
  AND ($2 = '' OR ar.profile = $2)
  AND ($3 = '' OR ar.status = $3)
ORDER BY ar.started_at DESC,ar.id DESC
LIMIT $4 OFFSET $5
`, environmentID, profile, status, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AuditRunRow, 0)
	for rows.Next() {
		var r AuditRunRow
		var finished pgtype.Timestamptz
		var warnings, errorsJSON []byte
		if err := rows.Scan(
			&r.ID, &r.EnvironmentID, &r.EnvironmentName, &r.Profile, &r.Status,
			&r.ServiceVersion, &r.CollectorVersion, &r.StartedAt, &finished, &warnings, &errorsJSON,
		); err != nil {
			return nil, err
		}
		if finished.Valid {
			t := finished.Time
			r.FinishedAt = &t
		}
		_ = json.Unmarshal(warnings, &r.Warnings)
		_ = json.Unmarshal(errorsJSON, &r.Errors)
		if r.Warnings == nil {
			r.Warnings = []string{}
		}
		if r.Errors == nil {
			r.Errors = []string{}
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) GetAuditRun(ctx context.Context, id string) (*AuditRunRow, error) {
	row := s.pool.QueryRow(ctx, `
SELECT ar.id::text, ar.environment_id::text, COALESCE(e.name, ''), ar.profile, ar.status,
  ar.service_version, ar.collector_version, ar.started_at, ar.finished_at, ar.warnings, ar.errors
FROM audit_run ar
LEFT JOIN audit_environment e ON e.id = ar.environment_id
WHERE ar.id = $1::uuid
`, id)
	var r AuditRunRow
	var finished pgtype.Timestamptz
	var warnings, errorsJSON []byte
	if err := row.Scan(
		&r.ID, &r.EnvironmentID, &r.EnvironmentName, &r.Profile, &r.Status,
		&r.ServiceVersion, &r.CollectorVersion, &r.StartedAt, &finished, &warnings, &errorsJSON,
	); err != nil {
		return nil, err
	}
	if finished.Valid {
		t := finished.Time
		r.FinishedAt = &t
	}
	_ = json.Unmarshal(warnings, &r.Warnings)
	_ = json.Unmarshal(errorsJSON, &r.Errors)
	if r.Warnings == nil {
		r.Warnings = []string{}
	}
	if r.Errors == nil {
		r.Errors = []string{}
	}
	return &r, nil
}

func (s *Store) ListCollectorRuns(ctx context.Context, auditRunID string) ([]CollectorRunRow, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id::text, audit_run_id::text, collector_name, collector_version, status,
  started_at, finished_at, rows_collected, warning, error
FROM collector_run
WHERE audit_run_id = $1::uuid
ORDER BY started_at
`, auditRunID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]CollectorRunRow, 0)
	for rows.Next() {
		var r CollectorRunRow
		var finished pgtype.Timestamptz
		var warning, errMsg pgtype.Text
		if err := rows.Scan(
			&r.ID, &r.AuditRunID, &r.CollectorName, &r.CollectorVersion, &r.Status,
			&r.StartedAt, &finished, &r.RowsCollected, &warning, &errMsg,
		); err != nil {
			return nil, err
		}
		if finished.Valid {
			t := finished.Time
			r.FinishedAt = &t
		}
		if warning.Valid {
			w := warning.String
			r.Warning = &w
		}
		if errMsg.Valid {
			e := errMsg.String
			r.Error = &e
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
