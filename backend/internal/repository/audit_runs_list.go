package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type AuditRunRow struct {
	ID               string     `json:"id"`
	EnvironmentID    string     `json:"environment_id"`
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

// ListAuditRuns returns recent runs, optionally filtered by environment/profile/status.
func (s *Store) ListAuditRuns(ctx context.Context, environmentID, profile, status string, limit int) ([]AuditRunRow, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
SELECT id::text, environment_id::text, profile, status, service_version, collector_version,
  started_at, finished_at, warnings, errors
FROM audit_run
WHERE ($1 = '' OR environment_id = $1::uuid)
  AND ($2 = '' OR profile = $2)
  AND ($3 = '' OR status = $3)
ORDER BY started_at DESC
LIMIT $4
`, environmentID, profile, status, limit)
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
			&r.ID, &r.EnvironmentID, &r.Profile, &r.Status, &r.ServiceVersion, &r.CollectorVersion,
			&r.StartedAt, &finished, &warnings, &errorsJSON,
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
	items, err := s.ListAuditRuns(ctx, "", "", "", 200)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].ID == id {
			return &items[i], nil
		}
	}
	// direct query fallback
	row := s.pool.QueryRow(ctx, `
SELECT id::text, environment_id::text, profile, status, service_version, collector_version,
  started_at, finished_at, warnings, errors
FROM audit_run WHERE id = $1::uuid
`, id)
	var r AuditRunRow
	var finished pgtype.Timestamptz
	var warnings, errorsJSON []byte
	if err := row.Scan(
		&r.ID, &r.EnvironmentID, &r.Profile, &r.Status, &r.ServiceVersion, &r.CollectorVersion,
		&r.StartedAt, &finished, &warnings, &errorsJSON,
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
