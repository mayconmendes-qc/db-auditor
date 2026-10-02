package repository

import (
	"context"
	"errors"
	"sort"

	"github.com/jackc/pgx/v5"
)

const scopeScoreVersion = "scope-v1"

type ScoreCategory struct {
	Category string `json:"category"`
	Score    int    `json:"score"`
	Penalty  int    `json:"penalty"`
	Findings int    `json:"findings"`
}

type ScopeScore struct {
	Version           string          `json:"version"`
	EnvironmentID     string          `json:"environment_id"`
	AuditRunID        string          `json:"audit_run_id"`
	DatabaseName      string          `json:"database_name,omitempty"`
	SchemaName        string          `json:"schema_name,omitempty"`
	TableName         string          `json:"table_name,omitempty"`
	Status            string          `json:"status"`
	Score             *int            `json:"score"`
	Confidence        float64         `json:"confidence"`
	Categories        []ScoreCategory `json:"categories"`
	MissingCollectors []string        `json:"missing_collectors"`
}

var scoreCollectors = []string{"postgres.tables", "postgres.columns", "postgres.constraints", "postgres.indexes"}

func severityPenalty(severity string) int {
	switch severity {
	case "critical":
		return 25
	case "high":
		return 15
	case "medium":
		return 8
	case "low":
		return 3
	default:
		return 1
	}
}

func calculateScopeCategories(observations [][2]string) ([]ScoreCategory, int) {
	byCategory := map[string]*ScoreCategory{}
	for _, o := range observations {
		category := o[0]
		if category == "" {
			category = "other"
		}
		item := byCategory[category]
		if item == nil {
			item = &ScoreCategory{Category: category, Score: 100}
			byCategory[category] = item
		}
		item.Findings++
		item.Penalty += severityPenalty(o[1])
		if item.Penalty > 100 {
			item.Penalty = 100
		}
		item.Score = 100 - item.Penalty
	}
	out := make([]ScoreCategory, 0, len(byCategory))
	sum := 0
	for _, v := range byCategory {
		out = append(out, *v)
		sum += v.Score
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Category < out[j].Category })
	if len(out) == 0 {
		return out, 100
	}
	return out, sum / len(out)
}

func (s *Store) GetScopeScore(ctx context.Context, environmentID, runID, database, schema, table string) (*ScopeScore, error) {
	var runStatus string
	err := s.pool.QueryRow(ctx, `SELECT status FROM audit_run WHERE id=$1::uuid AND environment_id=$2::uuid AND status IN ('success','partial_success')`, runID, environmentID).Scan(&runStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := &ScopeScore{Version: scopeScoreVersion, EnvironmentID: environmentID, AuditRunID: runID, DatabaseName: database, SchemaName: schema, TableName: table, Status: "available", Categories: []ScoreCategory{}, MissingCollectors: []string{}}
	rows, err := s.pool.Query(ctx, `SELECT database_name FROM database_snapshot WHERE audit_run_id=$1::uuid AND ($2='' OR database_name=$2) ORDER BY database_name`, runID, database)
	if err != nil {
		return nil, err
	}
	databases := []string{}
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			rows.Close()
			return nil, err
		}
		databases = append(databases, name)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if len(databases) == 0 {
		out.Status = "insufficient_coverage"
		out.MissingCollectors = []string{"postgres.databases"}
		return out, nil
	}
	rows, err = s.pool.Query(ctx, `SELECT database_name,collector_name FROM audit_run_coverage WHERE audit_run_id=$1::uuid AND ($2='' OR database_name=$2) AND status='success'`, runID, database)
	if err != nil {
		return nil, err
	}
	available := map[string]bool{}
	for rows.Next() {
		var db, name string
		if err = rows.Scan(&db, &name); err != nil {
			rows.Close()
			return nil, err
		}
		available[db+"/"+name] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for _, db := range databases {
		for _, name := range scoreCollectors {
			if !available[db+"/"+name] {
				out.MissingCollectors = append(out.MissingCollectors, db+"/"+name)
			}
		}
	}
	out.Confidence = float64(len(databases)*len(scoreCollectors)-len(out.MissingCollectors)) / float64(len(databases)*len(scoreCollectors))
	if runStatus != "success" {
		out.Confidence *= 0.75
	}
	rows, err = s.pool.Query(ctx, `SELECT category,severity FROM finding_event WHERE audit_run_id=$1::uuid AND event_type='observed' AND ($2='' OR database_name=$2) AND ($3='' OR schema_name=$3) AND ($4='' OR object_name=$4) ORDER BY category,severity`, runID, database, schema, table)
	if err != nil {
		return nil, err
	}
	observations := [][2]string{}
	for rows.Next() {
		var category, severity string
		if err = rows.Scan(&category, &severity); err != nil {
			rows.Close()
			return nil, err
		}
		observations = append(observations, [2]string{category, severity})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	var score int
	out.Categories, score = calculateScopeCategories(observations)
	if len(out.MissingCollectors) > 0 || runStatus != "success" {
		out.Status = "insufficient_coverage"
		return out, nil
	}
	out.Score = &score
	return out, nil
}
