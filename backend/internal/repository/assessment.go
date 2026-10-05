package repository

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5"
)

// TableAssessment is an immutable, run-scoped entry point. Collections remain
// paginated through the linked inventory endpoints instead of being embedded.
type TableAssessment struct {
	Version int               `json:"version"`
	Run     AssessmentRun     `json:"run"`
	Table   AssessmentTable   `json:"table"`
	Summary AssessmentSummary `json:"summary"`
	Links   map[string]string `json:"links"`
}

type AssessmentSummary struct {
	Columns                int                     `json:"columns"`
	Constraints            int                     `json:"constraints"`
	Indexes                int                     `json:"indexes"`
	Findings               int                     `json:"findings"`
	Grants                 int                     `json:"grants"`
	Dependencies           int                     `json:"dependencies"`
	Triggers               int                     `json:"triggers"`
	RLSPolicies            int                     `json:"rls_policies"`
	Score                  *float64                `json:"score"`
	ScoreStatus            string                  `json:"score_status"`
	ScoreVersion           string                  `json:"score_version"`
	ScoreConfidence        *float64                `json:"score_confidence"`
	ScoreFactors           []AssessmentScoreFactor `json:"score_factors"`
	ScoreMissingCollectors []string                `json:"score_missing_collectors"`
}

type AssessmentRun struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	StartedAt time.Time `json:"started_at"`
	Partial   bool      `json:"partial"`
}

type AssessmentTable struct {
	TableSnapshotRow
	ParentSchemaName  *string  `json:"parent_schema_name,omitempty"`
	ParentTableName   *string  `json:"parent_table_name,omitempty"`
	TablespaceName    *string  `json:"tablespace_name,omitempty"`
	Persistence       *string  `json:"persistence,omitempty"`
	RLSEnabled        bool     `json:"rls_enabled"`
	RLSForced         bool     `json:"rls_forced"`
	Comment           *string  `json:"comment,omitempty"`
	StorageParameters []string `json:"storage_parameters"`
	NLiveTup          int64    `json:"n_live_tup"`
	NDeadTup          int64    `json:"n_dead_tup"`
	SeqScan           int64    `json:"seq_scan"`
	IdxScan           int64    `json:"idx_scan"`
}

// GetTableAssessment never falls back to another run. A missing or unfinished
// run, or an object absent from that run, is reported as not found.
func (s *Store) GetTableAssessment(ctx context.Context, env, run, database, schema, table string) (*TableAssessment, error) {
	var result TableAssessment
	result.Version = 1
	err := s.pool.QueryRow(ctx, `
SELECT r.id::text, r.status, r.started_at,
  t.id::text, t.audit_run_id::text, t.environment_id::text,
  t.database_name, t.schema_name, t.table_name, t.owner_name,
  COALESCE(t.relkind,''), COALESCE(t.relation_class,''), COALESCE(t.is_partition,false),
  t.total_size_bytes, t.data_size_bytes, t.index_size_bytes,
  COALESCE(t.row_estimate,0), COALESCE(t.column_count,0), COALESCE(t.has_primary_key,false), t.collected_at,
  t.parent_schema_name, t.parent_table_name, t.tablespace_name, t.relpersistence,
  COALESCE(t.relrowsecurity,false), COALESCE(t.relforcerowsecurity,false), t.table_comment, COALESCE(t.storage_parameters,ARRAY[]::text[]),
  COALESCE(t.n_live_tup,0), COALESCE(t.n_dead_tup,0), COALESCE(t.seq_scan,0), COALESCE(t.idx_scan,0)
FROM audit_run r
JOIN table_snapshot t ON t.audit_run_id = r.id AND t.environment_id = r.environment_id
WHERE r.environment_id = $1::uuid AND r.id = $2::uuid
  AND r.status IN ('success','partial_success')
  AND t.database_name = $3 AND t.schema_name = $4 AND t.table_name = $5`,
		env, run, database, schema, table).Scan(
		&result.Run.ID, &result.Run.Status, &result.Run.StartedAt,
		&result.Table.ID, &result.Table.AuditRunID, &result.Table.EnvironmentID,
		&result.Table.DatabaseName, &result.Table.SchemaName, &result.Table.TableName, &result.Table.OwnerName,
		&result.Table.Relkind, &result.Table.RelationClass, &result.Table.IsPartition,
		&result.Table.TotalSizeBytes, &result.Table.DataSizeBytes, &result.Table.IndexSizeBytes,
		&result.Table.RowEstimate, &result.Table.ColumnCount, &result.Table.HasPrimaryKey, &result.Table.CollectedAt,
		&result.Table.ParentSchemaName, &result.Table.ParentTableName, &result.Table.TablespaceName, &result.Table.Persistence,
		&result.Table.RLSEnabled, &result.Table.RLSForced, &result.Table.Comment, &result.Table.StorageParameters,
		&result.Table.NLiveTup, &result.Table.NDeadTup, &result.Table.SeqScan, &result.Table.IdxScan,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("table assessment: %w", err)
	}
	result.Run.Partial = result.Run.Status == "partial_success"
	result.Summary.ScoreVersion = structuralScoreVersion
	result.Summary.ScoreFactors = []AssessmentScoreFactor{}
	result.Summary.ScoreMissingCollectors = []string{}
	var invalidIndexes, unvalidatedConstraints int
	err = s.pool.QueryRow(ctx, `
SELECT
  (SELECT count(*) FROM column_snapshot WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid AND database_name=$3 AND schema_name=$4 AND table_name=$5),
  (SELECT count(*) FROM constraint_snapshot WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid AND database_name=$3 AND schema_name=$4 AND table_name=$5),
  (SELECT count(*) FROM index_snapshot WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid AND database_name=$3 AND schema_name=$4 AND table_name=$5),
  (SELECT count(*) FROM finding WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid AND database_name=$3 AND schema_name=$4 AND (object_name=$5 OR evidence->>'table'=$5 OR evidence->>'table_name'=$5)),
  (SELECT count(*) FROM grant_snapshot WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid AND database_name=$3 AND schema_name=$4 AND table_name=$5),
  (SELECT count(*) FROM object_dependency_snapshot WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid AND database_name=$3 AND ((source_schema=$4 AND source_name=$5) OR (target_schema=$4 AND target_name=$5))),
  (SELECT count(*) FROM trigger_snapshot WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid AND database_name=$3 AND schema_name=$4 AND table_name=$5),
  (SELECT count(*) FROM rls_policy_snapshot WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid AND database_name=$3 AND schema_name=$4 AND table_name=$5),
  (SELECT count(*) FROM index_snapshot WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid AND database_name=$3 AND schema_name=$4 AND table_name=$5 AND NOT is_valid),
  (SELECT count(*) FROM constraint_snapshot WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid AND database_name=$3 AND schema_name=$4 AND table_name=$5 AND NOT is_validated)
`, env, run, database, schema, table).Scan(&result.Summary.Columns, &result.Summary.Constraints, &result.Summary.Indexes, &result.Summary.Findings,
		&result.Summary.Grants, &result.Summary.Dependencies, &result.Summary.Triggers, &result.Summary.RLSPolicies,
		&invalidIndexes, &unvalidatedConstraints)
	if err != nil {
		return nil, fmt.Errorf("table assessment summary: %w", err)
	}
	coverage, err := s.pool.Query(ctx, `
SELECT collector_name, status FROM audit_run_coverage
WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid AND database_name=$3
  AND collector_name=ANY($4::text[])`, env, run, database, structuralScoreCollectors)
	if err != nil {
		return nil, fmt.Errorf("table assessment score coverage: %w", err)
	}
	collectorStatuses := make(map[string]string, len(structuralScoreCollectors))
	for coverage.Next() {
		var collector, status string
		if err := coverage.Scan(&collector, &status); err != nil {
			coverage.Close()
			return nil, fmt.Errorf("table assessment score coverage: %w", err)
		}
		collectorStatuses[collector] = status
	}
	err = coverage.Err()
	coverage.Close()
	if err != nil {
		return nil, fmt.Errorf("table assessment score coverage: %w", err)
	}
	for _, collector := range structuralScoreCollectors {
		if collectorStatuses[collector] != "success" {
			result.Summary.ScoreMissingCollectors = append(result.Summary.ScoreMissingCollectors, collector)
		}
	}
	if len(result.Summary.ScoreMissingCollectors) > 0 {
		result.Summary.ScoreStatus = "insufficient_coverage"
	} else {
		score, factors := calculateStructuralScore(structuralScoreInput{
			RelationKind: result.Table.Relkind, IsPartition: result.Table.IsPartition,
			HasPrimaryKey: result.Table.HasPrimaryKey, LiveTuples: result.Table.NLiveTup,
			DeadTuples: result.Table.NDeadTup, InvalidIndexes: invalidIndexes,
			UnvalidatedConstraints: unvalidatedConstraints,
		})
		result.Summary.Score = &score
		result.Summary.ScoreFactors = factors
		result.Summary.ScoreStatus = "available"
		confidence := 1.0
		if result.Run.Partial {
			confidence = 0.75
		}
		result.Summary.ScoreConfidence = &confidence
	}
	base := "/api/v1/environments/" + url.PathEscape(env)
	q := url.Values{"audit_run_id": {run}, "database": {database}, "schema": {schema}, "table": {table}}
	result.Links = map[string]string{
		"columns":     base + "/columns?" + q.Encode(),
		"constraints": base + "/constraints?" + q.Encode(),
		"indexes":     base + "/indexes?" + q.Encode(),
		"findings":    base + "/runs/" + url.PathEscape(run) + "/databases/" + url.PathEscape(database) + "/schemas/" + url.PathEscape(schema) + "/tables/" + url.PathEscape(table) + "/findings",
		"history":     base + "/tables/history?" + q.Encode(),
		"graph":       base + "/runs/" + url.PathEscape(run) + "/databases/" + url.PathEscape(database) + "/schemas/" + url.PathEscape(schema) + "/tables/" + url.PathEscape(table) + "/graph",
	}
	for _, kind := range []string{"grants", "dependencies", "triggers", "rls-policies"} {
		result.Links[kind] = base + "/runs/" + url.PathEscape(run) + "/databases/" + url.PathEscape(database) + "/schemas/" + url.PathEscape(schema) + "/tables/" + url.PathEscape(table) + "/" + kind
	}
	return &result, nil
}

// ListTableFindings is intentionally scoped to the finding's recorded run.
// Historical findings are reconstructed from immutable observations, never
// from the mutable current status or the finding's most recent run pointer.
func (s *Store) ListTableFindings(ctx context.Context, env, run, database, schema, table string, limit, offset int) ([]Finding, int, error) {
	var total int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM finding_event e JOIN finding f ON f.id=e.finding_id
WHERE f.environment_id=$1::uuid AND e.audit_run_id=$2::uuid AND e.event_type='observed' AND e.database_name=$3
  AND e.schema_name=$4 AND (e.object_name=$5 OR e.evidence->>'table'=$5 OR e.evidence->>'table_name'=$5)`, env, run, database, schema, table).Scan(&total)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `
SELECT f.id::text, f.environment_id::text, e.audit_run_id::text,
  f.finding_type, e.severity, e.finding_status, e.title, e.summary,
  f.object_type, f.object_key, e.database_name, e.schema_name, e.object_name,
  e.evidence, f.dedup_key, f.rule_id, e.rule_version, e.category, e.confidence, e.impact, e.risk,
  e.recommendation, e.validation, e.reference_urls, e.rule_parameters, f.first_seen_at, e.recorded_at, NULL::timestamptz, NULL::text,
  f.created_at, e.recorded_at, 0, NULL::text, NULL::timestamptz, NULL::text, f.assignee, f.due_at
FROM finding_event e JOIN finding f ON f.id=e.finding_id
WHERE f.environment_id=$1::uuid AND e.audit_run_id=$2::uuid AND e.event_type='observed' AND e.database_name=$3
  AND e.schema_name=$4 AND (e.object_name=$5 OR e.evidence->>'table'=$5 OR e.evidence->>'table_name'=$5)
ORDER BY e.severity, f.id LIMIT $6 OFFSET $7`, env, run, database, schema, table, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]Finding, 0)
	for rows.Next() {
		f, err := scanFinding(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *f)
	}
	return out, total, rows.Err()
}

// ErrInvalidGraphLimit denotes a request outside the deliberately small graph budget.
var ErrInvalidGraphLimit = errors.New("invalid graph limit")

type RelationshipNode struct {
	Database string `json:"database"`
	Schema   string `json:"schema"`
	Table    string `json:"table"`
}

type RelationshipEdge struct {
	From           RelationshipNode `json:"from"`
	To             RelationshipNode `json:"to"`
	ConstraintName string           `json:"constraint_name"`
	Columns        []string         `json:"columns"`
	Referenced     []string         `json:"referenced_columns"`
}

type RelationshipGraph struct {
	Nodes     []RelationshipNode `json:"nodes"`
	Edges     []RelationshipEdge `json:"edges"`
	Truncated bool               `json:"truncated"`
}

// TableRelationshipGraph returns only direct, same-database foreign keys.
// The SQL LIMIT is applied before any graph expansion and protects large schemas.
func (s *Store) TableRelationshipGraph(ctx context.Context, env, run, database, schema, table string, limit int) (*RelationshipGraph, error) {
	if limit < 1 || limit > 100 {
		return nil, ErrInvalidGraphLimit
	}
	rows, err := s.pool.Query(ctx, `
SELECT schema_name, table_name, referenced_schema_name, referenced_table_name,
  constraint_name, COALESCE(constrained_columns,ARRAY[]::text[]), COALESCE(referenced_columns,ARRAY[]::text[])
FROM constraint_snapshot
WHERE environment_id = $1::uuid AND audit_run_id = $2::uuid AND database_name = $3
  AND constraint_type = 'f' AND referenced_schema_name IS NOT NULL AND referenced_table_name IS NOT NULL
  AND ((schema_name = $4 AND table_name = $5) OR (referenced_schema_name = $4 AND referenced_table_name = $5))
ORDER BY schema_name, table_name, constraint_name
LIMIT $6`, env, run, database, schema, table, limit+1)
	if err != nil {
		return nil, fmt.Errorf("table relationship graph: %w", err)
	}
	defer rows.Close()
	root := RelationshipNode{Database: database, Schema: schema, Table: table}
	g := &RelationshipGraph{Nodes: []RelationshipNode{root}, Edges: []RelationshipEdge{}}
	seen := map[RelationshipNode]bool{root: true}
	for rows.Next() {
		if len(g.Edges) == limit {
			g.Truncated = true
			break
		}
		var fromSchema, fromTable, toSchema, toTable, name string
		var cols, refs []string
		if err := rows.Scan(&fromSchema, &fromTable, &toSchema, &toTable, &name, &cols, &refs); err != nil {
			return nil, err
		}
		from := RelationshipNode{Database: database, Schema: fromSchema, Table: fromTable}
		to := RelationshipNode{Database: database, Schema: toSchema, Table: toTable}
		for _, node := range []RelationshipNode{from, to} {
			if !seen[node] {
				g.Nodes = append(g.Nodes, node)
				seen[node] = true
			}
		}
		g.Edges = append(g.Edges, RelationshipEdge{From: from, To: to, ConstraintName: name, Columns: cols, Referenced: refs})
	}
	return g, rows.Err()
}
