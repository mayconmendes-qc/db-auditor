package analyzer

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// IndexAnalyzer detects possibly unused indexes. It never recommends DROP.
type IndexAnalyzer struct{}

func (IndexAnalyzer) Name() string { return "index" }

func (IndexAnalyzer) Analyze(_ context.Context, facts SnapshotFacts) ([]Finding, error) {
	out := make([]Finding, 0)
	// index.unused is a two-collection comparison in P2Analyzer. A single
	// snapshot, including one taken on the day of stats_reset, is not enough.
	byDef := map[string][]IndexFact{}
	for _, idx := range facts.Indexes {
		if idx.HasValidity && (!idx.IsValid || !idx.IsReady) {
			continue
		}
		structure := indexStructure(idx)
		if structure == "" {
			continue
		}
		byDef[structure] = append(byDef[structure], idx)
	}
	for def, group := range byDef {
		if len(group) < 2 {
			continue
		}
		names := make([]string, 0, len(group))
		for _, g := range group {
			names = append(names, g.IndexName)
		}
		sort.Strings(names)
		key := fmt.Sprintf("%s.%s.dup:%s", group[0].Database, group[0].Schema, names[0])
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

func indexStructure(idx IndexFact) string {
	definition := strings.ToLower(strings.Join(strings.Fields(idx.Definition), " "))
	if definition == "" {
		return ""
	}
	if pos := strings.Index(definition, " on "); pos >= 0 {
		definition = definition[pos+4:]
	}
	return fmt.Sprintf("%s|%s|%s|unique=%t|primary=%t|%s", idx.Database, idx.Schema, idx.TableName, idx.IsUnique, idx.IsPrimary, definition)
}
