package analyzer

import (
	"context"
	"fmt"
	"time"
)

// CAGGAnalyzer diagnoses continuous aggregates.
type CAGGAnalyzer struct{}

func (CAGGAnalyzer) Name() string { return "cagg" }

func (CAGGAnalyzer) Analyze(_ context.Context, facts SnapshotFacts) ([]Finding, error) {
	out := make([]Finding, 0)
	for _, c := range facts.CAGGs {
		key := fmt.Sprintf("%s.%s.%s", c.Database, c.Schema, c.ViewName)
		if !c.HasRefreshPolicy {
			title := fmt.Sprintf("CAGG without refresh policy: %s", key)
			f := Finding{
				EnvironmentID: facts.EnvironmentID,
				AuditRunID:    facts.AuditRunID,
				FindingType:   "cagg.missing_refresh_policy",
				Severity:      SeverityHigh,
				Status:        StatusOpen,
				Title:         title,
				Summary:       "Continuous aggregate has no refresh policy. Materialization may lag or never update.",
				ObjectType:    "continuous_aggregate",
				ObjectKey:     key,
				DatabaseName:  c.Database,
				SchemaName:    c.Schema,
				ObjectName:    c.ViewName,
				Evidence: map[string]any{
					"materialization":   fmt.Sprintf("%s.%s", c.MaterializationSchema, c.MaterializationHypertable),
					"materialized_only": c.MaterializedOnly,
				},
				DedupKey: DedupKey("cagg.missing_refresh_policy", key, title),
			}
			out = append(out, f)
		}
		if c.HasRefreshPolicy && durationExceeds(c.Lag, time.Hour) {
			title := fmt.Sprintf("CAGG materialization lag: %s", key)
			out = append(out, Finding{
				EnvironmentID: facts.EnvironmentID,
				AuditRunID:    facts.AuditRunID,
				FindingType:   "cagg.materialization_lag",
				Severity:      SeverityHigh,
				Status:        StatusOpen,
				Title:         title,
				Summary:       "Continuous aggregate has a refresh policy and the materialized window is behind the server clock.",
				ObjectType:    "continuous_aggregate",
				ObjectKey:     key,
				DatabaseName:  c.Database,
				SchemaName:    c.Schema,
				ObjectName:    c.ViewName,
				Evidence:      map[string]any{"lag": c.Lag},
				DedupKey:      DedupKey("cagg.materialization_lag", key, title),
			})
		}
		if c.Realtime {
			title := fmt.Sprintf("CAGG real-time aggregation enabled: %s", key)
			out = append(out, Finding{
				EnvironmentID: facts.EnvironmentID,
				AuditRunID:    facts.AuditRunID,
				FindingType:   "cagg.realtime_hypothesis",
				Severity:      SeverityLow,
				Status:        StatusOpen,
				Title:         title,
				Summary:       "Real-time aggregation is enabled. This is a hypothesis, not a request to turn it off.",
				ObjectType:    "continuous_aggregate",
				ObjectKey:     key,
				DatabaseName:  c.Database,
				SchemaName:    c.Schema,
				ObjectName:    c.ViewName,
				Evidence:      map[string]any{"realtime": true, "confidence": "low"},
				DedupKey:      DedupKey("cagg.realtime_hypothesis", key, title),
			})
		}
	}
	// Potentially overlapping definitions (same view_definition text).
	byDef := map[string][]CAGGFact{}
	for _, c := range facts.CAGGs {
		if c.ViewDefinition == "" {
			continue
		}
		byDef[c.ViewDefinition] = append(byDef[c.ViewDefinition], c)
	}
	for def, group := range byDef {
		if len(group) < 2 {
			continue
		}
		names := make([]string, 0, len(group))
		for _, g := range group {
			names = append(names, fmt.Sprintf("%s.%s", g.Schema, g.ViewName))
		}
		key := fmt.Sprintf("%s.overlap:%s", group[0].Database, group[0].ViewName)
		title := fmt.Sprintf("Potentially overlapping CAGGs in %s", group[0].Database)
		f := Finding{
			EnvironmentID: facts.EnvironmentID,
			AuditRunID:    facts.AuditRunID,
			FindingType:   "cagg.overlap",
			Severity:      SeverityLow,
			Status:        StatusOpen,
			Title:         title,
			Summary:       "Multiple continuous aggregates share the same view definition. Review for redundancy.",
			ObjectType:    "continuous_aggregate",
			ObjectKey:     key,
			DatabaseName:  group[0].Database,
			SchemaName:    group[0].Schema,
			ObjectName:    group[0].ViewName,
			Evidence: map[string]any{
				"views":      names,
				"definition": def,
			},
			DedupKey: DedupKey("cagg.overlap", key, title),
		}
		out = append(out, f)
	}
	return out, nil
}
