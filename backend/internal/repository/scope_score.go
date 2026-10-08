package repository

import (
	"context"
	"errors"
	"sort"

	"github.com/jackc/pgx/v5"
)

const ScopeScoreFormulaVersion = "scope-v3"

type ScoreCategory struct {
	Category string `json:"category"`
	Weight   int    `json:"weight"`
	Score    int    `json:"score"`
	Penalty  int    `json:"penalty"`
	Positive int    `json:"positive"`
	Findings int    `json:"findings"`
}

type ScopeScore struct {
	Version           string          `json:"version"`
	Profile           string          `json:"profile"`
	CollectorVersion  string          `json:"collector_version"`
	RuleVersion       string          `json:"rule_version"`
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
	Explanation       string          `json:"explanation"`
}

var scoreWeights = map[string]int{"security": 35, "performance": 30, "structure": 20, "maintenance": 15}

func scoreGroup(category string) string {
	switch category {
	case "security", "privilege", "rls", "config":
		return "security"
	case "performance", "index", "workload":
		return "performance"
	case "storage", "vacuum", "policy", "job", "chunk", "cagg":
		return "maintenance"
	default:
		return "structure"
	}
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

func calculateScopeCategories(observations [][2]string, primaryKeys, tables int) ([]ScoreCategory, int) {
	byCategory := map[string]*ScoreCategory{}
	for name, weight := range scoreWeights {
		byCategory[name] = &ScoreCategory{Category: name, Weight: weight, Score: 100}
	}
	// A valid identifier on observed tables is affirmative structural evidence.
	// Missing identifiers cannot receive the same structural score as verified
	// identifiers, even if no analyzer finding was emitted.
	structure := byCategory["structure"]
	structure.Score = 80
	if tables > 0 {
		structure.Positive = 20 * primaryKeys / tables
		structure.Score += structure.Positive
	}
	for _, o := range observations {
		category := scoreGroup(o[0])
		item := byCategory[category]
		item.Findings++
		item.Penalty += severityPenalty(o[1])
		if item.Penalty > 100 {
			item.Penalty = 100
		}
		if category == "structure" {
			item.Score = 80 + item.Positive - item.Penalty
		} else {
			item.Score = 100 - item.Penalty
		}
		if item.Score < 0 {
			item.Score = 0
		}
	}
	out := make([]ScoreCategory, 0, len(byCategory))
	weighted := 0
	for _, v := range byCategory {
		out = append(out, *v)
		weighted += v.Score * v.Weight
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Category < out[j].Category })
	return out, weighted / 100
}

func (s *Store) GetScopeScore(ctx context.Context, environmentID, runID, database, schema, table string) (*ScopeScore, error) {
	var runStatus string
	var profile, collectorVersion, ruleVersion string
	err := s.pool.QueryRow(ctx, `SELECT r.status,r.profile,r.collector_version,COALESCE(NULLIF(a.rule_manifest_hash,''),a.analyzer_version,'')
FROM audit_run r LEFT JOIN analysis_run a ON a.audit_run_id=r.id
WHERE r.id=$1::uuid AND r.environment_id=$2::uuid AND r.status IN ('success','partial_success')`, runID, environmentID).Scan(&runStatus, &profile, &collectorVersion, &ruleVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := &ScopeScore{Version: ScopeScoreFormulaVersion, EnvironmentID: environmentID, AuditRunID: runID, DatabaseName: database, SchemaName: schema, TableName: table, Status: "available", Categories: []ScoreCategory{}, MissingCollectors: []string{}, Explanation: "Nota ponderada por segurança (35%), performance (30%), estrutura (20%) e manutenção (15%). Cada achado desconta pontos; chaves primárias verificadas acrescentam até 20 pontos à estrutura. Ausência de achados só conta com análise e cobertura completas."}
	out.Profile, out.CollectorVersion, out.RuleVersion = profile, collectorVersion, ruleVersion
	engine, err := s.GetEnvironmentEngine(ctx, environmentID)
	if err != nil {
		return nil, err
	}
	if engine != "postgresql" && engine != "timescaledb" {
		out.Status = "not_applicable"
		out.Explanation = "Nota não aplicável ao mecanismo " + engine + "; as regras e a fórmula atuais foram validadas para PostgreSQL e TimescaleDB. Consulte o inventário e as capacidades disponíveis."
		return out, nil
	}
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
	var primaryKeys, tableCount int
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE has_primary_key)::int,count(*)::int FROM table_snapshot WHERE audit_run_id=$1::uuid AND ($2='' OR database_name=$2) AND ($3='' OR schema_name=$3) AND ($4='' OR table_name=$4)`, runID, database, schema, table).Scan(&primaryKeys, &tableCount); err != nil {
		return nil, err
	}
	if tableCount == 0 {
		out.Status = "insufficient_coverage"
		out.MissingCollectors = append(out.MissingCollectors, "postgres.tables")
		return out, nil
	}
	var score int
	out.Categories, score = calculateScopeCategories(observations, primaryKeys, tableCount)
	var analysisStatus string
	if err = s.pool.QueryRow(ctx, `SELECT COALESCE((SELECT status FROM analysis_run WHERE audit_run_id=$1::uuid),'')`, runID).Scan(&analysisStatus); err != nil {
		return nil, err
	}
	if len(out.MissingCollectors) > 0 || runStatus != "success" || analysisStatus != "success" {
		out.Status = "insufficient_coverage"
		if analysisStatus != "success" {
			out.MissingCollectors = append(out.MissingCollectors, "analysis")
		}
		return out, nil
	}
	out.Score = &score
	return out, nil
}
