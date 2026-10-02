package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

var ErrBaselineIneligible = errors.New("baseline requires a completed, fully covered run in the same environment and scope")

type AuditBaseline struct {
	EnvironmentID string    `json:"environment_id"`
	DatabaseName  string    `json:"database_name"`
	SchemaName    string    `json:"schema_name"`
	TableName     string    `json:"table_name"`
	AuditRunID    string    `json:"audit_run_id"`
	SelectedBy    string    `json:"selected_by"`
	SelectedAt    time.Time `json:"selected_at"`
}

type BaselineComparison struct {
	EnvironmentID string `json:"environment_id"`
	DatabaseName  string `json:"database_name"`
	SchemaName    string `json:"schema_name"`
	TableName     string `json:"table_name"`
	BaselineRunID string `json:"baseline_run_id"`
	AuditRunID    string `json:"audit_run_id"`
	Status        string `json:"status"`
	AddedTables   int    `json:"added_tables"`
	RemovedTables int    `json:"removed_tables"`
	ChangedTables int    `json:"changed_tables"`
}

func (s *Store) GetAuditBaseline(ctx context.Context, environmentID, database, schema, table string) (*AuditBaseline, error) {
	var b AuditBaseline
	err := s.pool.QueryRow(ctx, `SELECT environment_id::text,database_name,schema_name,table_name,audit_run_id::text,selected_by,selected_at FROM audit_baseline WHERE environment_id=$1::uuid AND database_name=$2 AND schema_name=$3 AND table_name=$4`, environmentID, database, schema, table).Scan(&b.EnvironmentID, &b.DatabaseName, &b.SchemaName, &b.TableName, &b.AuditRunID, &b.SelectedBy, &b.SelectedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &b, err
}

func (s *Store) SelectAuditBaseline(ctx context.Context, environmentID, database, schema, table, runID, actor string) (*AuditBaseline, error) {
	if schema != "" && database == "" || table != "" && schema == "" {
		return nil, ErrBaselineIneligible
	}
	if actor == "" {
		actor = "local"
	}
	var eligible bool
	err := s.pool.QueryRow(ctx, `SELECT r.status='success' AND EXISTS (
  SELECT 1 FROM database_snapshot d WHERE d.audit_run_id=r.id AND ($3='' OR d.database_name=$3)
) AND NOT EXISTS (
  SELECT 1 FROM database_snapshot d WHERE d.audit_run_id=r.id AND ($3='' OR d.database_name=$3)
    AND NOT EXISTS(SELECT 1 FROM audit_run_coverage c WHERE c.audit_run_id=r.id AND c.collector_name='postgres.tables' AND c.database_name=d.database_name AND c.status='success')
) AND NOT EXISTS (
  SELECT 1 FROM audit_run_coverage c WHERE c.audit_run_id=r.id AND ($3='' OR c.database_name=$3 OR c.database_name='') AND c.status IN ('failed','skipped','attempted')
) AND ($4='' OR EXISTS(SELECT 1 FROM schema_snapshot s WHERE s.audit_run_id=r.id AND s.database_name=$3 AND s.schema_name=$4))
AND ($5='' OR EXISTS(SELECT 1 FROM table_snapshot t WHERE t.audit_run_id=r.id AND t.database_name=$3 AND t.schema_name=$4 AND t.table_name=$5))
FROM audit_run r WHERE r.id=$1::uuid AND r.environment_id=$2::uuid`, runID, environmentID, database, schema, table).Scan(&eligible)
	if errors.Is(err, pgx.ErrNoRows) || !eligible {
		return nil, ErrBaselineIneligible
	}
	if err != nil {
		return nil, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var previous *string
	err = tx.QueryRow(ctx, `SELECT audit_run_id::text FROM audit_baseline WHERE environment_id=$1::uuid AND database_name=$2 AND schema_name=$3 AND table_name=$4 FOR UPDATE`, environmentID, database, schema, table).Scan(&previous)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var b AuditBaseline
	err = tx.QueryRow(ctx, `INSERT INTO audit_baseline(environment_id,database_name,schema_name,table_name,audit_run_id,selected_by) VALUES($1::uuid,$2,$3,$4,$5::uuid,$6)
ON CONFLICT(environment_id,database_name,schema_name,table_name) DO UPDATE SET audit_run_id=EXCLUDED.audit_run_id,selected_by=EXCLUDED.selected_by,selected_at=now()
RETURNING environment_id::text,database_name,schema_name,table_name,audit_run_id::text,selected_by,selected_at`, environmentID, database, schema, table, runID, actor).Scan(&b.EnvironmentID, &b.DatabaseName, &b.SchemaName, &b.TableName, &b.AuditRunID, &b.SelectedBy, &b.SelectedAt)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO baseline_selection_event(environment_id,database_name,schema_name,table_name,previous_run_id,selected_run_id,selected_by) VALUES($1::uuid,$2,$3,$4,$5::uuid,$6::uuid,$7)`, environmentID, database, schema, table, previous, runID, actor)
	if err != nil {
		return nil, err
	}
	return &b, tx.Commit(ctx)
}

func (s *Store) ListBaselineComparisons(ctx context.Context, environmentID, runID string) ([]BaselineComparison, error) {
	rows, err := s.pool.Query(ctx, `SELECT environment_id::text,database_name,schema_name,table_name,baseline_run_id::text,audit_run_id::text,status,added_tables,removed_tables,changed_tables
FROM baseline_comparison WHERE environment_id=$1::uuid AND ($2='' OR audit_run_id=$2::uuid) ORDER BY compared_at DESC,database_name,schema_name,table_name LIMIT 100`, environmentID, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BaselineComparison{}
	for rows.Next() {
		var x BaselineComparison
		if err = rows.Scan(&x.EnvironmentID, &x.DatabaseName, &x.SchemaName, &x.TableName, &x.BaselineRunID, &x.AuditRunID, &x.Status, &x.AddedTables, &x.RemovedTables, &x.ChangedTables); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// ReconcileCompletedRun is idempotent. A partial run retains diagnostic data
// but can never imply a missing table or a resolved finding.
func (s *Store) ReconcileCompletedRun(ctx context.Context, environmentID, runID string) error {
	var status, analysis string
	if err := s.pool.QueryRow(ctx, `SELECT r.status,COALESCE(a.status,'') FROM audit_run r LEFT JOIN analysis_run a ON a.audit_run_id=r.id WHERE r.id=$1::uuid AND r.environment_id=$2::uuid`, runID, environmentID).Scan(&status, &analysis); err != nil {
		return err
	}
	if status != "success" && status != "partial_success" {
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `SELECT database_name,schema_name,table_name,audit_run_id::text FROM audit_baseline WHERE environment_id=$1::uuid`, environmentID)
	if err != nil {
		return err
	}
	baselines := []AuditBaseline{}
	for rows.Next() {
		var b AuditBaseline
		if err = rows.Scan(&b.DatabaseName, &b.SchemaName, &b.TableName, &b.AuditRunID); err != nil {
			rows.Close()
			return err
		}
		baselines = append(baselines, b)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, b := range baselines {
		if b.AuditRunID == runID {
			continue
		}
		var complete bool
		err = tx.QueryRow(ctx, `SELECT $3='success' AND EXISTS (SELECT 1 FROM database_snapshot d WHERE d.audit_run_id=$1::uuid AND ($2='' OR d.database_name=$2)) AND NOT EXISTS (SELECT 1 FROM database_snapshot d WHERE d.audit_run_id=$1::uuid AND ($2='' OR d.database_name=$2) AND NOT EXISTS (SELECT 1 FROM audit_run_coverage c WHERE c.audit_run_id=$1::uuid AND c.database_name=d.database_name AND c.collector_name='postgres.tables' AND c.status='success')) AND NOT EXISTS (SELECT 1 FROM audit_run_coverage WHERE audit_run_id=$1::uuid AND ($2='' OR database_name=$2 OR database_name='') AND status IN ('failed','skipped','attempted'))`, runID, b.DatabaseName, status).Scan(&complete)
		if err != nil {
			return err
		}
		comparisonStatus := "partial"
		if complete {
			comparisonStatus = "complete"
		} else {
			var compatible bool
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM database_snapshot WHERE audit_run_id=$1::uuid AND ($2='' OR database_name=$2))`, runID, b.DatabaseName).Scan(&compatible); err != nil {
				return err
			}
			if !compatible {
				comparisonStatus = "incompatible"
			}
		}
		var added, removed, changed int
		if complete {
			err = tx.QueryRow(ctx, `WITH src AS (SELECT database_name,schema_name,table_name,relkind,has_primary_key,column_count FROM table_snapshot WHERE audit_run_id=$1::uuid AND ($3='' OR database_name=$3) AND ($4='' OR schema_name=$4) AND ($5='' OR table_name=$5)),
dst AS (SELECT database_name,schema_name,table_name,relkind,has_primary_key,column_count FROM table_snapshot WHERE audit_run_id=$2::uuid AND ($3='' OR database_name=$3) AND ($4='' OR schema_name=$4) AND ($5='' OR table_name=$5))
SELECT count(*) FILTER (WHERE src.table_name IS NULL),count(*) FILTER (WHERE dst.table_name IS NULL),count(*) FILTER (WHERE src.table_name IS NOT NULL AND dst.table_name IS NOT NULL AND (src.relkind,src.has_primary_key,src.column_count) IS DISTINCT FROM (dst.relkind,dst.has_primary_key,dst.column_count))
FROM src FULL JOIN dst USING(database_name,schema_name,table_name)`, b.AuditRunID, runID, b.DatabaseName, b.SchemaName, b.TableName).Scan(&added, &removed, &changed)
			if err != nil {
				return fmt.Errorf("compare baseline: %w", err)
			}
		}
		_, err = tx.Exec(ctx, `INSERT INTO baseline_comparison(environment_id,database_name,schema_name,table_name,baseline_run_id,audit_run_id,status,added_tables,removed_tables,changed_tables) VALUES($1::uuid,$2,$3,$4,$5::uuid,$6::uuid,$7,$8,$9,$10)
ON CONFLICT(audit_run_id,database_name,schema_name,table_name) DO UPDATE SET baseline_run_id=EXCLUDED.baseline_run_id,status=EXCLUDED.status,added_tables=EXCLUDED.added_tables,removed_tables=EXCLUDED.removed_tables,changed_tables=EXCLUDED.changed_tables,compared_at=now()`, environmentID, b.DatabaseName, b.SchemaName, b.TableName, b.AuditRunID, runID, comparisonStatus, added, removed, changed)
		if err != nil {
			return err
		}
	}
	if status == "success" && analysis == "success" {
		rows, err = tx.Query(ctx, `UPDATE finding f SET status='resolved',resolved_at=now(),updated_at=now()
WHERE f.environment_id=$1::uuid AND f.status IN ('open','acknowledged') AND f.audit_run_id IS NOT NULL
AND f.last_seen_at<(SELECT started_at FROM audit_run WHERE id=$2::uuid)
AND NOT EXISTS(SELECT 1 FROM finding_event e WHERE e.finding_id=f.id AND e.audit_run_id=$2::uuid AND e.event_type='observed')
AND EXISTS(SELECT 1 FROM audit_run_coverage c WHERE c.audit_run_id=$2::uuid AND c.collector_name='postgres.tables' AND c.database_name=f.database_name AND c.status='success')
AND NOT EXISTS(SELECT 1 FROM audit_run_coverage c WHERE c.audit_run_id=$2::uuid AND (c.database_name=f.database_name OR c.database_name='') AND c.status IN ('failed','skipped','attempted'))
AND NOT EXISTS(SELECT 1 FROM audit_run_coverage prior WHERE prior.audit_run_id=f.audit_run_id AND (prior.database_name=f.database_name OR prior.database_name='') AND prior.status='success'
  AND NOT EXISTS(SELECT 1 FROM audit_run_coverage current WHERE current.audit_run_id=$2::uuid AND current.collector_name=prior.collector_name AND current.database_name=prior.database_name AND current.status='success'))
RETURNING f.id::text`, environmentID, runID)
		if err != nil {
			return err
		}
		ids := []string{}
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, id := range ids {
			if _, err = tx.Exec(ctx, `INSERT INTO finding_event(finding_id,audit_run_id,event_type,reason) VALUES($1::uuid,$2::uuid,'resolved','absent from complete run') ON CONFLICT DO NOTHING`, id, runID); err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}
