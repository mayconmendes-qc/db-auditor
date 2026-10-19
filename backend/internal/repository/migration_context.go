package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/osmendes/db-auditor/internal/analyzer"
	"github.com/osmendes/db-auditor/internal/migrationrisk"
)

// MigrationTableContext is one table's snapshot metadata for the CLI.
type MigrationTableContext struct {
	Name         string  `json:"name"`
	SizeBytes    int64   `json:"size_bytes"`
	RowEstimate  int64   `json:"row_estimate"`
	WriteRate    float64 `json:"write_rate_per_s"`
	HasWriteRate bool    `json:"has_write_rate"`
}

// MigrationContextResponse is the API payload for GET migration-context.
type MigrationContextResponse struct {
	Partial          bool                    `json:"partial"`
	Stale            bool                    `json:"stale"`
	Missing          bool                    `json:"missing"`
	AuditRunID       string                  `json:"audit_run_id,omitempty"`
	AuditRunAt       string                  `json:"audit_run_at,omitempty"`
	ServerVersionNum int                     `json:"server_version_num"`
	AgeSeconds       int64                   `json:"age_seconds,omitempty"`
	Tables           []MigrationTableContext `json:"tables"`
}

// GetMigrationContext loads table sizes/rows/write-rate from the latest complete-ish snapshot.
func (s *Store) GetMigrationContext(ctx context.Context, environmentID string, tables []string, now time.Time) (*MigrationContextResponse, error) {
	if err := migrationrisk.ValidateTablesBound(tables); err != nil {
		return nil, err
	}
	out := &MigrationContextResponse{Tables: []MigrationTableContext{}}
	completeness, err := s.GetSnapshotCompleteness(ctx, environmentID, "")
	if err != nil {
		return nil, err
	}
	if completeness.Completeness == "empty" || completeness.AuditRunID == "" {
		out.Missing = true
		out.Partial = true
		return out, nil
	}
	out.AuditRunID = completeness.AuditRunID
	out.AuditRunAt = completeness.StartedAt
	out.Partial = completeness.Completeness == "partial" || strings.EqualFold(completeness.RunStatus, "partial_success")
	if completeness.StartedAt != "" {
		if ts, err := time.Parse(time.RFC3339, completeness.StartedAt); err == nil {
			age := now.Sub(ts)
			out.AgeSeconds = int64(age.Seconds())
			if age > 7*24*time.Hour {
				out.Stale = true
				out.Partial = true
			}
		}
	}
	// server_version from settings snapshot when present.
	var verText string
	_ = s.pool.QueryRow(ctx, `
SELECT setting_value FROM server_setting_snapshot
WHERE audit_run_id=$1::uuid AND setting_name='server_version' LIMIT 1`, completeness.AuditRunID).Scan(&verText)
	out.ServerVersionNum = parseServerVersionNum(verText)

	if len(tables) == 0 {
		return out, nil
	}
	prevRunID, prevStarted, err := s.previousCompleteRun(ctx, environmentID, completeness.AuditRunID)
	if err != nil {
		return nil, err
	}

	for _, raw := range tables {
		schema, name := splitTable(raw)
		var size, rows, nIns, nUpd, nDel int64
		err := s.pool.QueryRow(ctx, `
SELECT COALESCE(total_size_bytes,0), COALESCE(row_estimate,0),
       COALESCE(n_tup_ins,0), COALESCE(n_tup_upd,0), COALESCE(n_tup_del,0)
FROM table_snapshot
WHERE audit_run_id=$1::uuid AND schema_name=$2 AND table_name=$3
LIMIT 1`, completeness.AuditRunID, schema, name).Scan(&size, &rows, &nIns, &nUpd, &nDel)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		tc := MigrationTableContext{
			Name: schema + "." + name, SizeBytes: size, RowEstimate: rows,
		}
		if prevRunID != "" && !prevStarted.IsZero() {
			var pIns, pUpd, pDel int64
			err := s.pool.QueryRow(ctx, `
SELECT COALESCE(n_tup_ins,0), COALESCE(n_tup_upd,0), COALESCE(n_tup_del,0)
FROM table_snapshot
WHERE audit_run_id=$1::uuid AND schema_name=$2 AND table_name=$3
LIMIT 1`, prevRunID, schema, name).Scan(&pIns, &pUpd, &pDel)
			if err == nil {
				curStarted, _ := time.Parse(time.RFC3339, completeness.StartedAt)
				window := curStarted.Sub(prevStarted)
				if window > 0 && window <= analyzer.MaxComparableWindow {
					delta := float64((nIns + nUpd + nDel) - (pIns + pUpd + pDel))
					if delta < 0 {
						delta = 0
					}
					tc.WriteRate = delta / window.Seconds()
					tc.HasWriteRate = true
				}
			}
		}
		out.Tables = append(out.Tables, tc)
	}
	return out, nil
}

func (s *Store) previousCompleteRun(ctx context.Context, environmentID, currentID string) (string, time.Time, error) {
	var id string
	var started time.Time
	err := s.pool.QueryRow(ctx, `
SELECT id::text, started_at FROM audit_run
WHERE environment_id=$1::uuid AND status='success' AND id<>$2::uuid
ORDER BY started_at DESC, id DESC LIMIT 1`, environmentID, currentID).Scan(&id, &started)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", time.Time{}, nil
	}
	return id, started.UTC(), err
}

func splitTable(raw string) (schema, name string) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	parts := strings.Split(raw, ".")
	if len(parts) == 1 {
		return "public", parts[0]
	}
	return parts[0], parts[len(parts)-1]
}

func parseServerVersionNum(v string) int {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	// "16.4 (Debian ...)" → 160004 approximate major*10000
	major := 0
	for i := 0; i < len(v); i++ {
		if v[i] < '0' || v[i] > '9' {
			break
		}
		major = major*10 + int(v[i]-'0')
	}
	if major >= 14 && major <= 18 {
		return major * 10000
	}
	return 0
}
