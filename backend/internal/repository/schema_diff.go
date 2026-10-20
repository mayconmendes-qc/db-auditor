package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/osmendes/db-auditor/internal/comparison"
	"github.com/osmendes/db-auditor/internal/schemadiff"
)

// ErrMissingCompleteRun is returned when schema diff has no complete audit run to compare.
var ErrMissingCompleteRun = errors.New("missing complete audit run")

// latestCompleteRunID returns the newest successful complete analyzed run.
func (s *Store) latestCompleteRunID(ctx context.Context, environmentID string) (string, time.Time, error) {
	rows, err := s.pool.Query(ctx, `
SELECT r.id::text, r.started_at
FROM audit_run r
LEFT JOIN analysis_run a ON a.audit_run_id = r.id
WHERE r.environment_id=$1::uuid
  AND r.status = 'success'
  AND COALESCE(a.status, '') = 'success'
ORDER BY r.started_at DESC
LIMIT 20`, environmentID)
	if err != nil {
		return "", time.Time{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var started time.Time
		if err := rows.Scan(&id, &started); err != nil {
			return "", time.Time{}, err
		}
		cov, err := s.GetSnapshotCompleteness(ctx, environmentID, id)
		if err != nil {
			return "", time.Time{}, err
		}
		if cov != nil && cov.Completeness == "complete" {
			return id, started, nil
		}
	}
	return "", time.Time{}, rows.Err()
}

// SchemaDiffRequest selects two runs (same or different environments).
type SchemaDiffRequest struct {
	SourceEnv  string
	SourceRun  string // empty = latest complete for source env
	TargetEnv  string // empty = same as source
	TargetRun  string // empty = latest complete for target env
	ObjectType string
	Status     string // MATCH|DRIFT|ONLY_SOURCE|ONLY_TARGET|UNKNOWN or empty
	Limit      int
	Offset     int
}

// SchemaDiffResult is the API payload for a real schema diff.
type SchemaDiffResult struct {
	SourceEnvironmentID string                  `json:"source_environment_id"`
	TargetEnvironmentID string                  `json:"target_environment_id"`
	SourceRunID         string                  `json:"source_run_id"`
	TargetRunID         string                  `json:"target_run_id"`
	Partial             bool                    `json:"partial"`
	Summary             comparison.Summary      `json:"summary"`
	Objects             []comparison.ObjectDiff `json:"objects"`
	Total               int                     `json:"total"`
	Limit               int                     `json:"limit"`
	Offset              int                     `json:"offset"`
}

// ResolveSchemaDiffRuns picks default complete runs when ids are omitted.
func (s *Store) ResolveSchemaDiffRuns(ctx context.Context, req SchemaDiffRequest) (sourceRun, targetRun string, partial bool, err error) {
	targetEnv := req.TargetEnv
	if targetEnv == "" {
		targetEnv = req.SourceEnv
	}
	sourceRun = req.SourceRun
	if sourceRun == "" {
		sourceRun, _, err = s.latestCompleteRunID(ctx, req.SourceEnv)
		if err != nil {
			return "", "", false, err
		}
	}
	targetRun = req.TargetRun
	if targetRun == "" {
		targetRun, _, err = s.latestCompleteRunID(ctx, targetEnv)
		if err != nil {
			return "", "", false, err
		}
	}
	if sourceRun == "" || targetRun == "" {
		return sourceRun, targetRun, false, ErrMissingCompleteRun
	}
	srcCov, err := s.GetSnapshotCompleteness(ctx, req.SourceEnv, sourceRun)
	if err != nil {
		return "", "", false, err
	}
	tgtCov, err := s.GetSnapshotCompleteness(ctx, targetEnv, targetRun)
	if err != nil {
		return "", "", false, err
	}
	partial = (srcCov != nil && srcCov.Completeness == "partial") ||
		(tgtCov != nil && tgtCov.Completeness == "partial") ||
		(srcCov != nil && strings.EqualFold(srcCov.RunStatus, "partial_success")) ||
		(tgtCov != nil && strings.EqualFold(tgtCov.RunStatus, "partial_success"))
	return sourceRun, targetRun, partial, nil
}

// BuildSchemaDiff loads snapshot objects for both runs and compares them.
func (s *Store) BuildSchemaDiff(ctx context.Context, req SchemaDiffRequest) (*SchemaDiffResult, error) {
	targetEnv := req.TargetEnv
	if targetEnv == "" {
		targetEnv = req.SourceEnv
	}
	srcEngine, err := s.GetEnvironmentEngine(ctx, req.SourceEnv)
	if err != nil {
		return nil, err
	}
	tgtEngine, err := s.GetEnvironmentEngine(ctx, targetEnv)
	if err != nil {
		return nil, err
	}
	if !sameMechanism(srcEngine, tgtEngine) {
		return nil, schemadiff.ErrMechanismMismatch
	}

	sourceRun, targetRun, partial, err := s.ResolveSchemaDiffRuns(ctx, req)
	if err != nil {
		return nil, err
	}

	srcObjs, err := s.loadSchemaDiffObjects(ctx, req.SourceEnv, sourceRun)
	if err != nil {
		return nil, err
	}
	tgtObjs, err := s.loadSchemaDiffObjects(ctx, targetEnv, targetRun)
	if err != nil {
		return nil, err
	}

	mappings, err := s.schemaDiffMappings(ctx, req.SourceEnv, targetEnv)
	if err != nil {
		return nil, err
	}

	raw := schemadiff.Diff(srcObjs, tgtObjs, mappings)
	filtered := raw.Objects
	if req.ObjectType != "" {
		filtered = comparison.FilterByObjectType(filtered, req.ObjectType)
	}
	if req.Status != "" {
		filtered = comparison.FilterByStatus(filtered, []string{req.Status})
	}
	total := len(filtered)
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	page := filtered[offset:end]

	return &SchemaDiffResult{
		SourceEnvironmentID: req.SourceEnv,
		TargetEnvironmentID: targetEnv,
		SourceRunID:         sourceRun,
		TargetRunID:         targetRun,
		Partial:             partial,
		Summary:             summarizeDiffObjects(filtered),
		Objects:             page,
		Total:               total,
		Limit:               limit,
		Offset:              offset,
	}, nil
}

func summarizeDiffObjects(objects []comparison.ObjectDiff) comparison.Summary {
	var s comparison.Summary
	for _, o := range objects {
		s.Total++
		switch o.Status {
		case comparison.StatusMatch:
			s.Match++
		case comparison.StatusDrift:
			s.Drift++
		case comparison.StatusOnlySource:
			s.OnlySource++
		case comparison.StatusOnlyTarget:
			s.OnlyTarget++
		default:
			s.Unknown++
		}
	}
	return s
}

func sameMechanism(a, b string) bool {
	na := strings.ToLower(strings.TrimSpace(a))
	nb := strings.ToLower(strings.TrimSpace(b))
	if na == "" || nb == "" {
		return na == nb
	}
	return na == nb
}


func (s *Store) schemaDiffMappings(ctx context.Context, sourceEnv, targetEnv string) ([]schemadiff.NameMapping, error) {
	if sourceEnv == targetEnv {
		return nil, nil
	}
	items, err := s.ListObjectMappings(ctx, sourceEnv, targetEnv, "validated")
	if err != nil {
		return nil, err
	}
	out := make([]schemadiff.NameMapping, 0, len(items))
	for _, m := range items {
		src := comparison.KeyFor(m.SourceDatabase, m.SourceSchema, m.SourceObjectName)
		tgt := comparison.KeyFor(m.TargetDatabase, m.TargetSchema, m.TargetObjectName)
		out = append(out, schemadiff.NameMapping{
			ObjectType: m.SourceObjectType,
			SourceKey:  src,
			TargetKey:  tgt,
		})
	}
	return out, nil
}

func (s *Store) loadSchemaDiffObjects(ctx context.Context, env, run string) (schemadiff.SnapshotObjects, error) {
	var out schemadiff.SnapshotObjects

	tableRows, err := s.pool.Query(ctx, `
SELECT database_name, schema_name, table_name, COALESCE(relkind,''), has_primary_key, column_count
FROM table_snapshot WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid
ORDER BY database_name, schema_name, table_name`, env, run)
	if err != nil {
		return out, err
	}
	defer tableRows.Close()
	for tableRows.Next() {
		var t schemadiff.TableObj
		if err := tableRows.Scan(&t.Database, &t.Schema, &t.Name, &t.Relkind, &t.HasPrimaryKey, &t.ColumnCount); err != nil {
			return out, err
		}
		out.Tables = append(out.Tables, t)
	}
	if err := tableRows.Err(); err != nil {
		return out, err
	}

	colRows, err := s.pool.Query(ctx, `
SELECT database_name, schema_name, table_name, column_name, data_type, is_nullable,
       COALESCE(column_default,''), is_generated
FROM column_snapshot WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid
ORDER BY database_name, schema_name, table_name, ordinal_position`, env, run)
	if err != nil {
		return out, err
	}
	defer colRows.Close()
	for colRows.Next() {
		var c schemadiff.ColumnObj
		var def string
		if err := colRows.Scan(&c.Database, &c.Schema, &c.Table, &c.Name, &c.DataType, &c.IsNullable, &def, &c.IsGenerated); err != nil {
			return out, err
		}
		c.DefaultFingerprint = schemadiff.FingerprintValue(def)
		out.Columns = append(out.Columns, c)
	}
	if err := colRows.Err(); err != nil {
		return out, err
	}

	idxRows, err := s.pool.Query(ctx, `
SELECT database_name, schema_name, table_name, index_name, COALESCE(access_method,''),
       is_unique, is_primary, COALESCE(array_to_string(key_columns, ','), ''), COALESCE(predicate,'')
FROM index_snapshot WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid
ORDER BY database_name, schema_name, table_name, index_name`, env, run)
	if err != nil {
		return out, err
	}
	defer idxRows.Close()
	for idxRows.Next() {
		var i schemadiff.IndexObj
		var keys, pred string
		if err := idxRows.Scan(&i.Database, &i.Schema, &i.Table, &i.Name, &i.AccessMethod, &i.IsUnique, &i.IsPrimary, &keys, &pred); err != nil {
			return out, err
		}
		i.DefinitionFingerprint = schemadiff.FingerprintValue(keys + "\x1f" + pred + "\x1f" + i.AccessMethod)
		out.Indexes = append(out.Indexes, i)
	}
	if err := idxRows.Err(); err != nil {
		return out, err
	}

	viewRows, err := s.pool.Query(ctx, `
SELECT database_name, schema_name, view_name, COALESCE(relkind,'')
FROM view_snapshot WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid
ORDER BY database_name, schema_name, view_name`, env, run)
	if err != nil {
		return out, err
	}
	defer viewRows.Close()
	for viewRows.Next() {
		var v schemadiff.ViewObj
		if err := viewRows.Scan(&v.Database, &v.Schema, &v.Name, &v.Relkind); err != nil {
			return out, err
		}
		// Definition body lives in detail tables; inventory fingerprint is name+relkind.
		v.DefinitionFingerprint = schemadiff.FingerprintValue(v.Relkind)
		out.Views = append(out.Views, v)
	}
	if err := viewRows.Err(); err != nil {
		return out, err
	}

	fnRows, err := s.pool.Query(ctx, `
SELECT database_name, schema_name, function_name, identity_arguments,
       COALESCE(language_name,''), is_security_definer, COALESCE(function_definition,'')
FROM function_snapshot WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid
ORDER BY database_name, schema_name, function_name, identity_arguments`, env, run)
	if err != nil {
		return out, err
	}
	defer fnRows.Close()
	for fnRows.Next() {
		var f schemadiff.FunctionObj
		var def string
		if err := fnRows.Scan(&f.Database, &f.Schema, &f.Name, &f.IdentityArgs, &f.Language, &f.IsSecurityDefiner, &def); err != nil {
			return out, err
		}
		if def != "" {
			f.DefinitionFingerprint = schemadiff.FingerprintValue(def)
		} else {
			f.DefinitionFingerprint = schemadiff.FingerprintValue(f.Language + "\x1f" + fmt.Sprintf("%v", f.IsSecurityDefiner))
		}
		out.Functions = append(out.Functions, f)
	}
	if err := fnRows.Err(); err != nil {
		return out, err
	}

	return out, nil
}
