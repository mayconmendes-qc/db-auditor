package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// ScopeAggregate is one schema or database score derived from covered tables.
// It uses the same category penalties as scope-v2. Missing collection or
// analysis nulls the score instead of reporting 100.
type ScopeAggregate struct {
	DatabaseName      string   `json:"database_name"`
	SchemaName        string   `json:"schema_name,omitempty"`
	Status            string   `json:"status"`
	Score             *int     `json:"score"`
	Confidence        float64  `json:"confidence"`
	Tables            int      `json:"tables"`
	MissingCollectors []string `json:"missing_collectors"`
}

// AggregateScopeScores groups table observations. tables is the number of
// covered tables in the scope. missingIndexes forces insufficient_coverage
// and a null score. partial applies the 0.75 confidence factor.
func AggregateScopeScores(database, schema string, observations [][2]string, tables int, missingIndexes bool, partial bool) ScopeAggregate {
	out := ScopeAggregate{
		DatabaseName:      database,
		SchemaName:        schema,
		Status:            "available",
		Tables:            tables,
		MissingCollectors: []string{},
		Confidence:        1,
	}
	if tables == 0 {
		out.Status = "insufficient_coverage"
		out.MissingCollectors = []string{"postgres.tables"}
		out.Confidence = 0
		return out
	}
	if missingIndexes {
		out.Status = "insufficient_coverage"
		out.MissingCollectors = []string{"postgres.indexes"}
		out.Confidence = 0
		return out
	}
	_, score := calculateScopeCategories(observations)
	if partial {
		out.Confidence = 0.75
	}
	out.Score = &score
	return out
}

// ListScopeAggregates rolls structural findings up by schema. A schema whose
// index collector did not succeed gets a null score, never 100.
func (s *Store) ListScopeAggregates(ctx context.Context, environmentID, runID string) ([]ScopeAggregate, error) {
	var runStatus string
	err := s.pool.QueryRow(ctx, `SELECT status FROM audit_run WHERE id=$1::uuid AND environment_id=$2::uuid AND status IN ('success','partial_success')`, runID, environmentID).Scan(&runStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return []ScopeAggregate{}, nil
	}
	if err != nil {
		return nil, err
	}
	var analyzed bool
	if err = s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM analysis_run WHERE audit_run_id=$1::uuid AND status='success')`, runID).Scan(&analyzed); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
SELECT database_name, schema_name, count(*)
FROM table_snapshot
WHERE audit_run_id=$1::uuid
GROUP BY database_name, schema_name
ORDER BY database_name, schema_name`, runID)
	if err != nil {
		return nil, err
	}
	type key struct{ db, schema string }
	tables := map[key]int{}
	order := []key{}
	for rows.Next() {
		var db, schema string
		var n int
		if err = rows.Scan(&db, &schema, &n); err != nil {
			rows.Close()
			return nil, err
		}
		k := key{db, schema}
		tables[k] = n
		order = append(order, k)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	covered := map[string]bool{}
	rows, err = s.pool.Query(ctx, `SELECT database_name FROM audit_run_coverage WHERE audit_run_id=$1::uuid AND collector_name='postgres.indexes' AND status='success'`, runID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var db string
		if err = rows.Scan(&db); err != nil {
			rows.Close()
			return nil, err
		}
		covered[db] = true
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	obs := map[key][][2]string{}
	rows, err = s.pool.Query(ctx, `SELECT database_name, schema_name, category, severity FROM finding_event WHERE audit_run_id=$1::uuid AND event_type='observed'`, runID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var db, schema, category, severity string
		if err = rows.Scan(&db, &schema, &category, &severity); err != nil {
			rows.Close()
			return nil, err
		}
		k := key{db, schema}
		obs[k] = append(obs[k], [2]string{category, severity})
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	partial := runStatus != "success"
	out := make([]ScopeAggregate, 0, len(order))
	for _, k := range order {
		item := AggregateScopeScores(k.db, k.schema, obs[k], tables[k], !covered[k.db], partial)
		if !analyzed {
			item.Status = "insufficient_coverage"
			item.MissingCollectors = append(item.MissingCollectors, "analysis")
			item.Score = nil
			item.Confidence = 0
		}
		out = append(out, item)
	}
	return out, nil
}
