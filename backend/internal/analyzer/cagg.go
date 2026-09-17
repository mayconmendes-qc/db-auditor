package analyzer

import (
	"context"
	"fmt"
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
					"materialization": fmt.Sprintf("%s.%s", c.MaterializationSchema, c.MaterializationHypertable),
					"materialized_only": c.MaterializedOnly,
				},
				DedupKey: DedupKey("cagg.missing_refresh_policy", key, title),
			}
			out = append(out, f)
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
