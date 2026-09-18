package analyzer

import (
	"context"
	"fmt"
)

// VacuumAnalyzer flags tables with high dead-tuple ratios for manual vacuum review.
// It never recommends DROP or destructive maintenance.
type VacuumAnalyzer struct{}

func (VacuumAnalyzer) Name() string { return "vacuum" }

const (
	vacuumDeadRatioThreshold = 0.20 // 20% dead tuples
	vacuumMinLiveTuples      = 1000 // ignore tiny tables
)

func (VacuumAnalyzer) Analyze(_ context.Context, facts SnapshotFacts) ([]Finding, error) {
	out := make([]Finding, 0)
	for _, v := range facts.Vacuum {
		total := v.NLiveTup + v.NDeadTup
		if total < vacuumMinLiveTuples || v.NLiveTup <= 0 {
			continue
		}
		ratio := float64(v.NDeadTup) / float64(total)
		if ratio < vacuumDeadRatioThreshold {
			continue
		}
		key := fmt.Sprintf("%s.%s.%s", v.Database, v.Schema, v.Name)
		title := fmt.Sprintf("High dead-tuple ratio: %s", key)
		sev := SeverityLow
		if ratio >= 0.40 {
			sev = SeverityMedium
		}
		f := Finding{
			EnvironmentID: facts.EnvironmentID,
			AuditRunID:    facts.AuditRunID,
			FindingType:   "vacuum.high_dead_tuples",
			Severity:      sev,
			Status:        StatusOpen,
			Title:         title,
			Summary: fmt.Sprintf(
				"Table has %.0f%% dead tuples (%d dead / %d total). Review autovacuum settings; no automatic VACUUM is issued.",
				ratio*100, v.NDeadTup, total,
			),
			ObjectType:   "table",
			ObjectKey:    key,
			DatabaseName: v.Database,
			SchemaName:   v.Schema,
			ObjectName:   v.Name,
			Evidence: map[string]any{
				"n_live_tup":      v.NLiveTup,
				"n_dead_tup":      v.NDeadTup,
				"dead_ratio":      ratio,
				"threshold":       vacuumDeadRatioThreshold,
				"last_vacuum":     v.LastVacuum,
				"last_autovacuum": v.LastAutovacuum,
				"safety_note":     "Manual review only; auditor never runs VACUUM or DROP",
			},
			DedupKey: DedupKey("vacuum.high_dead_tuples", key, title),
		}
		out = append(out, f)
	}
	return out, nil
}
