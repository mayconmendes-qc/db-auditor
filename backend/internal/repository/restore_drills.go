package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/osmendes/db-auditor/internal/restoreverify"
)

// TargetRestoreDrill is one recorded restore drill for an audited database.
type TargetRestoreDrill struct {
	ID                 string          `json:"id"`
	EnvironmentID      string          `json:"environment_id"`
	Kind               string          `json:"kind"`
	ArtifactSHA256     string          `json:"artifact_sha256"`
	Succeeded          bool            `json:"succeeded"`
	RTOSeconds         *int            `json:"rto_seconds,omitempty"`
	PrevalidateSeconds *int            `json:"prevalidate_seconds,omitempty"`
	VerifySeconds      *int            `json:"verify_seconds,omitempty"`
	Checks             json.RawMessage `json:"checks"`
	Divergences        json.RawMessage `json:"divergences"`
	CLIVersion         string          `json:"cli_version"`
	Note               string          `json:"note,omitempty"`
	TokenID            *string         `json:"token_id,omitempty"`
	PerformedAt        time.Time       `json:"performed_at"`
	RecordedAt         time.Time       `json:"recorded_at"`
}

// TargetRestorePolicy is the per-environment RTO target and drill max age.
type TargetRestorePolicy struct {
	EnvironmentID    string    `json:"environment_id"`
	RTOTargetSeconds int       `json:"rto_target_seconds"`
	MaxAgeDays       int       `json:"max_age_days"`
	UpdatedBy        string    `json:"updated_by,omitempty"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// RestoreDrillContext is the CLI-facing context (no DSNs).
type RestoreDrillContext struct {
	SourceMajor        int                              `json:"source_major"`
	AuditRunID         string                           `json:"audit_run_id,omitempty"`
	TargetFingerprints []string                         `json:"target_fingerprints"`
	Inventory          []restoreverify.InventoryObject  `json:"inventory"`
	EstimatedRows      []restoreverify.TableEstimate    `json:"estimated_rows"`
	WriteCounters      []restoreverify.TableWriteCounter `json:"write_counters"`
	Partial            bool                             `json:"partial"`
}

// RecordTargetRestoreDrillInput is the validated POST body.
type RecordTargetRestoreDrillInput struct {
	Kind               string          `json:"kind"`
	ArtifactSHA256     string          `json:"artifact_sha256"`
	Succeeded          bool            `json:"succeeded"`
	RTOSeconds         *int            `json:"rto_seconds"`
	PrevalidateSeconds *int            `json:"prevalidate_seconds"`
	VerifySeconds      *int            `json:"verify_seconds"`
	Checks             json.RawMessage `json:"checks"`
	Divergences        json.RawMessage `json:"divergences"`
	CLIVersion         string          `json:"cli_version"`
	Note               string          `json:"note"`
	PerformedAt        time.Time       `json:"performed_at"`
}

func (s *Store) tableExists(ctx context.Context, name string) bool {
	var ok bool
	_ = s.pool.QueryRow(ctx, `SELECT EXISTS (
SELECT 1 FROM information_schema.tables
WHERE table_schema='public' AND table_name=$1)`, name).Scan(&ok)
	return ok
}

// GetRestoreDrillContext builds fingerprints + inventory for the CLI.
// fingerprints must already be hashed host:port/db (never raw DSNs).
func (s *Store) GetRestoreDrillContext(ctx context.Context, environmentID string, fingerprints []string) (*RestoreDrillContext, error) {
	out := &RestoreDrillContext{
		TargetFingerprints: fingerprints,
		Inventory:          []restoreverify.InventoryObject{},
		EstimatedRows:      []restoreverify.TableEstimate{},
		WriteCounters:      []restoreverify.TableWriteCounter{},
	}
	completeness, err := s.GetSnapshotCompleteness(ctx, environmentID, "")
	if err != nil {
		return nil, err
	}
	if completeness.Completeness == "empty" || completeness.AuditRunID == "" {
		out.Partial = true
		return out, nil
	}
	out.AuditRunID = completeness.AuditRunID
	out.Partial = completeness.Completeness == "partial" || strings.EqualFold(completeness.RunStatus, "partial_success")

	var verText string
	_ = s.pool.QueryRow(ctx, `
SELECT setting_value FROM server_setting_snapshot
WHERE audit_run_id=$1::uuid AND setting_name='server_version' LIMIT 1`, completeness.AuditRunID).Scan(&verText)
	out.SourceMajor = parseServerVersionMajor(verText)

	rows, err := s.pool.Query(ctx, `
SELECT 'table', schema_name||'.'||table_name, table_name
FROM table_snapshot WHERE audit_run_id=$1::uuid
UNION ALL
SELECT 'index', schema_name||'.'||index_name, index_name
FROM index_snapshot WHERE audit_run_id=$1::uuid
UNION ALL
SELECT 'function', schema_name||'.'||function_name, function_name
FROM function_snapshot WHERE audit_run_id=$1::uuid
ORDER BY 1,2`, completeness.AuditRunID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var o restoreverify.InventoryObject
			if scanErr := rows.Scan(&o.ObjectType, &o.Key, &o.Name); scanErr != nil {
				break
			}
			out.Inventory = append(out.Inventory, o)
		}
	}

	trows, err := s.pool.Query(ctx, `
SELECT schema_name, table_name, COALESCE(row_estimate,0),
       COALESCE(n_tup_ins,0), COALESCE(n_tup_upd,0), COALESCE(n_tup_del,0)
FROM table_snapshot WHERE audit_run_id=$1::uuid`, completeness.AuditRunID)
	if err == nil {
		defer trows.Close()
		for trows.Next() {
			var est restoreverify.TableEstimate
			var wc restoreverify.TableWriteCounter
			if scanErr := trows.Scan(&est.Schema, &est.Table, &est.RowEstimate, &wc.NTupIns, &wc.NTupUpd, &wc.NTupDel); scanErr != nil {
				break
			}
			est.Comparable = true
			wc.Schema, wc.Table = est.Schema, est.Table
			out.EstimatedRows = append(out.EstimatedRows, est)
			out.WriteCounters = append(out.WriteCounters, wc)
		}
	}
	return out, nil
}

func parseServerVersionMajor(v string) int {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	// "16.4" or "16.4 (Ubuntu ...)"
	maj := v
	for i, c := range v {
		if c < '0' || c > '9' {
			maj = v[:i]
			break
		}
	}
	n, _ := strconv.Atoi(maj)
	return n
}

// RecordTargetRestoreDrill inserts evidence. Refuses row-value fields.
func (s *Store) RecordTargetRestoreDrill(ctx context.Context, environmentID string, tokenID *string, in RecordTargetRestoreDrillInput) (*TargetRestoreDrill, error) {
	if !s.tableExists(ctx, "target_restore_drill") {
		return nil, fmt.Errorf("migration 39 ausente")
	}
	if in.Kind != "" && in.Kind != restoreverify.KindTargetRestoreDrill {
		return nil, fmt.Errorf("kind inválido")
	}
	if err := restoreverify.RefuseRowValues(in.Checks); err != nil {
		return nil, err
	}
	if err := restoreverify.RefuseRowValues(in.Divergences); err != nil {
		return nil, err
	}
	sha := strings.ToLower(strings.TrimSpace(in.ArtifactSHA256))
	if len(sha) != 64 {
		return nil, fmt.Errorf("artifact_sha256 inválido")
	}
	if in.Checks == nil {
		in.Checks = json.RawMessage("[]")
	}
	if in.Divergences == nil {
		in.Divergences = json.RawMessage("[]")
	}
	if in.PerformedAt.IsZero() {
		in.PerformedAt = time.Now().UTC()
	}
	var out TargetRestoreDrill
	err := s.pool.QueryRow(ctx, `
INSERT INTO target_restore_drill (
  environment_id, artifact_sha256, succeeded, rto_seconds, prevalidate_seconds, verify_seconds,
  checks, divergences, cli_version, note, token_id, performed_at)
VALUES ($1::uuid,$2,$3,$4,$5,$6,$7::jsonb,$8::jsonb,$9,$10,$11::uuid,$12)
RETURNING id::text, environment_id::text, kind, artifact_sha256, succeeded, rto_seconds,
  prevalidate_seconds, verify_seconds, checks, divergences, cli_version, note,
  token_id::text, performed_at, recorded_at`,
		environmentID, sha, in.Succeeded, in.RTOSeconds, in.PrevalidateSeconds, in.VerifySeconds,
		string(in.Checks), string(in.Divergences), in.CLIVersion, in.Note, tokenID, in.PerformedAt.UTC(),
	).Scan(&out.ID, &out.EnvironmentID, &out.Kind, &out.ArtifactSHA256, &out.Succeeded, &out.RTOSeconds,
		&out.PrevalidateSeconds, &out.VerifySeconds, &out.Checks, &out.Divergences, &out.CLIVersion, &out.Note,
		&out.TokenID, &out.PerformedAt, &out.RecordedAt)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// ListTargetRestoreDrills returns newest-first history.
func (s *Store) ListTargetRestoreDrills(ctx context.Context, environmentID string, limit, offset int) ([]TargetRestoreDrill, int, error) {
	if !s.tableExists(ctx, "target_restore_drill") {
		return []TargetRestoreDrill{}, 0, nil
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM target_restore_drill WHERE environment_id=$1::uuid`, environmentID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `
SELECT id::text, environment_id::text, kind, artifact_sha256, succeeded, rto_seconds,
  prevalidate_seconds, verify_seconds, checks, divergences, cli_version, note,
  token_id::text, performed_at, recorded_at
FROM target_restore_drill WHERE environment_id=$1::uuid
ORDER BY performed_at DESC LIMIT $2 OFFSET $3`, environmentID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []TargetRestoreDrill{}
	for rows.Next() {
		var d TargetRestoreDrill
		if err := rows.Scan(&d.ID, &d.EnvironmentID, &d.Kind, &d.ArtifactSHA256, &d.Succeeded, &d.RTOSeconds,
			&d.PrevalidateSeconds, &d.VerifySeconds, &d.Checks, &d.Divergences, &d.CLIVersion, &d.Note,
			&d.TokenID, &d.PerformedAt, &d.RecordedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, d)
	}
	return items, total, rows.Err()
}

// GetTargetRestoreDrill returns one drill by id.
func (s *Store) GetTargetRestoreDrill(ctx context.Context, environmentID, drillID string) (*TargetRestoreDrill, error) {
	if !s.tableExists(ctx, "target_restore_drill") {
		return nil, pgx.ErrNoRows
	}
	var d TargetRestoreDrill
	err := s.pool.QueryRow(ctx, `
SELECT id::text, environment_id::text, kind, artifact_sha256, succeeded, rto_seconds,
  prevalidate_seconds, verify_seconds, checks, divergences, cli_version, note,
  token_id::text, performed_at, recorded_at
FROM target_restore_drill WHERE environment_id=$1::uuid AND id=$2::uuid`, environmentID, drillID).
		Scan(&d.ID, &d.EnvironmentID, &d.Kind, &d.ArtifactSHA256, &d.Succeeded, &d.RTOSeconds,
			&d.PrevalidateSeconds, &d.VerifySeconds, &d.Checks, &d.Divergences, &d.CLIVersion, &d.Note,
			&d.TokenID, &d.PerformedAt, &d.RecordedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// GetTargetRestorePolicy returns policy or defaults when missing.
func (s *Store) GetTargetRestorePolicy(ctx context.Context, environmentID string) (*TargetRestorePolicy, error) {
	def := &TargetRestorePolicy{
		EnvironmentID: environmentID, RTOTargetSeconds: 600, MaxAgeDays: restoreverify.DefaultMaxAgeDays,
		UpdatedAt: time.Now().UTC(),
	}
	if !s.tableExists(ctx, "target_restore_policy") {
		return def, nil
	}
	var p TargetRestorePolicy
	err := s.pool.QueryRow(ctx, `
SELECT environment_id::text, rto_target_seconds, max_age_days, updated_by, updated_at
FROM target_restore_policy WHERE environment_id=$1::uuid`, environmentID).
		Scan(&p.EnvironmentID, &p.RTOTargetSeconds, &p.MaxAgeDays, &p.UpdatedBy, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return def, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// UpsertTargetRestorePolicy stores operator RTO target and max age.
func (s *Store) UpsertTargetRestorePolicy(ctx context.Context, environmentID, actor string, rtoTarget, maxAgeDays int) (*TargetRestorePolicy, error) {
	if !s.tableExists(ctx, "target_restore_policy") {
		return nil, fmt.Errorf("migration 39 ausente")
	}
	if rtoTarget <= 0 || maxAgeDays <= 0 {
		return nil, fmt.Errorf("rto_target_seconds e max_age_days devem ser > 0")
	}
	var p TargetRestorePolicy
	err := s.pool.QueryRow(ctx, `
INSERT INTO target_restore_policy (environment_id, rto_target_seconds, max_age_days, updated_by, updated_at)
VALUES ($1::uuid,$2,$3,$4,now())
ON CONFLICT (environment_id) DO UPDATE SET
  rto_target_seconds=EXCLUDED.rto_target_seconds,
  max_age_days=EXCLUDED.max_age_days,
  updated_by=EXCLUDED.updated_by,
  updated_at=now()
RETURNING environment_id::text, rto_target_seconds, max_age_days, updated_by, updated_at`,
		environmentID, rtoTarget, maxAgeDays, actor).
		Scan(&p.EnvironmentID, &p.RTOTargetSeconds, &p.MaxAgeDays, &p.UpdatedBy, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// LastSuccessfulTargetRestoreDrill returns the newest succeeded drill, or nil.
func (s *Store) LastSuccessfulTargetRestoreDrill(ctx context.Context, environmentID string) (*TargetRestoreDrill, error) {
	if !s.tableExists(ctx, "target_restore_drill") {
		return nil, nil
	}
	var d TargetRestoreDrill
	err := s.pool.QueryRow(ctx, `
SELECT id::text, environment_id::text, kind, artifact_sha256, succeeded, rto_seconds,
  prevalidate_seconds, verify_seconds, checks, divergences, cli_version, note,
  token_id::text, performed_at, recorded_at
FROM target_restore_drill
WHERE environment_id=$1::uuid AND succeeded=true
ORDER BY performed_at DESC LIMIT 1`, environmentID).
		Scan(&d.ID, &d.EnvironmentID, &d.Kind, &d.ArtifactSHA256, &d.Succeeded, &d.RTOSeconds,
			&d.PrevalidateSeconds, &d.VerifySeconds, &d.Checks, &d.Divergences, &d.CLIVersion, &d.Note,
			&d.TokenID, &d.PerformedAt, &d.RecordedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}
