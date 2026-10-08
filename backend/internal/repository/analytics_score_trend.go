package repository

import (
	"context"
	"time"
)

func trendIDs(points []RunTrendPoint) []string {
	ids := make([]string, 0, len(points))
	for _, point := range points {
		ids = append(ids, point.AuditRunID)
	}
	return ids
}

func (s *Store) markTrendResets(ctx context.Context, points []RunTrendPoint) error {
	if len(points) == 0 {
		return nil
	}
	rows, err := s.pool.Query(ctx, `SELECT audit_run_id::text,max(stats_reset) FROM table_snapshot
WHERE audit_run_id=ANY($1::uuid[]) GROUP BY audit_run_id`, trendIDs(points))
	if err != nil {
		return err
	}
	resets := map[string]*time.Time{}
	for rows.Next() {
		var id string
		var at *time.Time
		if err = rows.Scan(&id, &at); err != nil {
			rows.Close()
			return err
		}
		resets[id] = at
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	previous := map[string]*time.Time{}
	for i := range points {
		point := &points[i]
		current := resets[point.AuditRunID]
		if prior := previous[point.EnvironmentID]; prior != nil && current != nil && !prior.Equal(*current) {
			point.CounterReset = true
			point.Comparable = false
			point.ComparisonNote = "Contadores reiniciados entre coletas"
		}
		previous[point.EnvironmentID] = current
	}
	return nil
}

// addTrendScores applies the same versioned formula as GetScopeScore to all
// runs in two bounded queries. A missing collector keeps the score absent.
func (s *Store) addTrendScores(ctx context.Context, points []RunTrendPoint) error {
	if len(points) == 0 {
		return nil
	}
	ids := trendIDs(points)
	rows, err := s.pool.Query(ctx, `SELECT audit_run_id::text,category,severity FROM finding_event
WHERE audit_run_id=ANY($1::uuid[]) AND event_type='observed' ORDER BY audit_run_id,category,severity`, ids)
	if err != nil {
		return err
	}
	observations := map[string][][2]string{}
	for rows.Next() {
		var id, category, severity string
		if err = rows.Scan(&id, &category, &severity); err != nil {
			rows.Close()
			return err
		}
		observations[id] = append(observations[id], [2]string{category, severity})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	rows, err = s.pool.Query(ctx, `SELECT d.audit_run_id::text,count(*)::int,
sum((EXISTS(SELECT 1 FROM audit_run_coverage c WHERE c.audit_run_id=d.audit_run_id AND c.database_name=d.database_name AND c.collector_name='postgres.tables' AND c.status='success'))::int
+(EXISTS(SELECT 1 FROM audit_run_coverage c WHERE c.audit_run_id=d.audit_run_id AND c.database_name=d.database_name AND c.collector_name='postgres.columns' AND c.status='success'))::int
+(EXISTS(SELECT 1 FROM audit_run_coverage c WHERE c.audit_run_id=d.audit_run_id AND c.database_name=d.database_name AND c.collector_name='postgres.constraints' AND c.status='success'))::int
+(EXISTS(SELECT 1 FROM audit_run_coverage c WHERE c.audit_run_id=d.audit_run_id AND c.database_name=d.database_name AND c.collector_name='postgres.indexes' AND c.status='success'))::int)::int
FROM database_snapshot d WHERE d.audit_run_id=ANY($1::uuid[]) AND NOT d.is_template GROUP BY d.audit_run_id`, ids)
	if err != nil {
		return err
	}
	type coverage struct{ databases, collectors int }
	covered := map[string]coverage{}
	for rows.Next() {
		var id string
		var item coverage
		if err = rows.Scan(&id, &item.databases, &item.collectors); err != nil {
			rows.Close()
			return err
		}
		covered[id] = item
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	tableRows, err := s.pool.Query(ctx, `SELECT audit_run_id::text,count(*) FILTER (WHERE has_primary_key)::int,count(*)::int FROM table_snapshot WHERE audit_run_id=ANY($1::uuid[]) GROUP BY audit_run_id`, ids)
	if err != nil {
		return err
	}
	type structuralEvidence struct{ primaryKeys, tables int }
	evidence := map[string]structuralEvidence{}
	for tableRows.Next() {
		var id string
		var item structuralEvidence
		if err = tableRows.Scan(&id, &item.primaryKeys, &item.tables); err != nil {
			tableRows.Close()
			return err
		}
		evidence[id] = item
	}
	err = tableRows.Err()
	tableRows.Close()
	if err != nil {
		return err
	}
	for i := range points {
		point := &points[i]
		point.ScoreVersion = ScopeScoreFormulaVersion
		coverage := covered[point.AuditRunID]
		if coverage.databases == 0 {
			continue
		}
		point.ScoreConfidence = float64(coverage.collectors) / float64(coverage.databases*len(scoreCollectors))
		if point.Coverage != "complete" || point.Status != "success" || coverage.collectors != coverage.databases*len(scoreCollectors) || evidence[point.AuditRunID].tables == 0 {
			continue
		}
		facts := evidence[point.AuditRunID]
		_, score := calculateScopeCategories(observations[point.AuditRunID], facts.primaryKeys, facts.tables)
		point.Score = &score
	}
	return nil
}
