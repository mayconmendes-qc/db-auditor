package analyzer

import (
	"context"
	"fmt"
)

// IndexAnalyzer detects possibly unused indexes. It never recommends DROP.
type IndexAnalyzer struct{}

func (IndexAnalyzer) Name() string { return "index" }

func (IndexAnalyzer) Analyze(_ context.Context, facts SnapshotFacts) ([]Finding, error) {
	out := make([]Finding, 0)
	for _, idx := range facts.Indexes {
		if idx.IsPrimary || idx.IsUnique {
			continue
		}
		if idx.IdxScan > 0 {
			continue
		}
		key := fmt.Sprintf("%s.%s.%s", idx.Database, idx.Schema, idx.IndexName)
		title := fmt.Sprintf("Possibly unused index: %s", key)
		f := Finding{
			EnvironmentID: facts.EnvironmentID,
			AuditRunID:    facts.AuditRunID,
			FindingType:   "index.unused",
			Severity:      SeverityMedium,
			Status:        StatusOpen,
			Title:         title,
			Summary:       "Index reports idx_scan=0. Manual review required; no automatic DROP is suggested.",
			ObjectType:    "index",
			ObjectKey:     key,
			DatabaseName:  idx.Database,
			SchemaName:    idx.Schema,
			ObjectName:    idx.IndexName,
			Evidence: map[string]any{
				"table_name": idx.TableName,
				"idx_scan":   idx.IdxScan,
				"size_bytes": idx.SizeBytes,
				"note":       "Never recommend DROP automatically",
			},
			DedupKey: DedupKey("index.unused", key, title),
		}
		out = append(out, f)
	}
	// Overlapping / duplicate detection by identical definition text.
	byDef := map[string][]IndexFact{}
	for _, idx := range facts.Indexes {
		if idx.Definition == "" {
			continue
		}
		byDef[idx.Definition] = append(byDef[idx.Definition], idx)
	}
	for def, group := range byDef {
		if len(group) < 2 {
			continue
		}
		names := make([]string, 0, len(group))
		for _, g := range group {
			names = append(names, g.IndexName)
		}
		key := fmt.Sprintf("%s.%s.dup:%s", group[0].Database, group[0].Schema, group[0].IndexName)
		title := fmt.Sprintf("Possibly overlapping indexes on %s.%s", group[0].Schema, group[0].TableName)
		f := Finding{
			EnvironmentID: facts.EnvironmentID,
			AuditRunID:    facts.AuditRunID,
			FindingType:   "index.overlap",
			Severity:      SeverityLow,
			Status:        StatusOpen,
			Title:         title,
			Summary:       "Multiple indexes share the same definition text. Review for redundancy.",
			ObjectType:    "index",
			ObjectKey:     key,
			DatabaseName:  group[0].Database,
			SchemaName:    group[0].Schema,
			ObjectName:    group[0].TableName,
			Evidence: map[string]any{
				"index_names": names,
				"definition":  def,
			},
			DedupKey: DedupKey("index.overlap", key, title),
		}
		out = append(out, f)
	}
	return out, nil
}
