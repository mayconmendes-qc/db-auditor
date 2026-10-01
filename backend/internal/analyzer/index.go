package analyzer

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

const minimumIndexObservation = 30 * 24 * time.Hour

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
		if idx.StatsReset == nil || idx.CollectedAt.IsZero() || idx.CollectedAt.Sub(*idx.StatsReset) < minimumIndexObservation {
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
				"table_name":       idx.TableName,
				"idx_scan":         idx.IdxScan,
				"size_bytes":       idx.SizeBytes,
				"observation_days": int(idx.CollectedAt.Sub(*idx.StatsReset).Hours() / 24),
				"stats_reset":      idx.StatsReset.UTC().Format(time.RFC3339),
				"note":             "Never recommend DROP automatically",
			},
			DedupKey: DedupKey("index.unused", key, title),
		}
		out = append(out, f)
	}
	// Duplicate detection compares the indexed structure, excluding the index name.
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
