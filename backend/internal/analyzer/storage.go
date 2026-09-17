package analyzer

import (
	"context"
	"fmt"
	"sort"
)

// StorageAnalyzer identifies large tables and simple growth signals.
type StorageAnalyzer struct{}

func (StorageAnalyzer) Name() string { return "storage" }

func (StorageAnalyzer) Analyze(_ context.Context, facts SnapshotFacts) ([]Finding, error) {
	out := make([]Finding, 0)
	if len(facts.Tables) == 0 {
		return out, nil
	}
	tables := append([]TableFact(nil), facts.Tables...)
	sort.Slice(tables, func(i, j int) bool {
		return tables[i].SizeBytes > tables[j].SizeBytes
	})
	topN := 5
	if len(tables) < topN {
		topN = len(tables)
	}
	for i := 0; i < topN; i++ {
		t := tables[i]
		if t.SizeBytes <= 0 {
			continue
		}
		key := fmt.Sprintf("%s.%s.%s", t.Database, t.Schema, t.Name)
		title := fmt.Sprintf("Top storage consumer: %s", key)
		f := Finding{
			EnvironmentID: facts.EnvironmentID,
			AuditRunID:    facts.AuditRunID,
			FindingType:   "storage.top_consumer",
			Severity:      SeverityInfo,
			Status:        StatusOpen,
			Title:         title,
			Summary:       fmt.Sprintf("Table ranks among top consumers by size (%d bytes).", t.SizeBytes),
			ObjectType:    "table",
			ObjectKey:     key,
			DatabaseName:  t.Database,
			SchemaName:    t.Schema,
			ObjectName:    t.Name,
			Evidence: map[string]any{
				"size_bytes": t.SizeBytes,
				"rank":       i + 1,
			},
			DedupKey: DedupKey("storage.top_consumer", key, title),
		}
		out = append(out, f)
	}
	// Simple growth: tables > 1 GiB flagged as review candidates.
	const oneGiB int64 = 1 << 30
	for _, t := range tables {
		if t.SizeBytes < oneGiB {
			continue
		}
		key := fmt.Sprintf("%s.%s.%s", t.Database, t.Schema, t.Name)
		title := fmt.Sprintf("Large table: %s", key)
		f := Finding{
			EnvironmentID: facts.EnvironmentID,
			AuditRunID:    facts.AuditRunID,
			FindingType:   "storage.large_table",
			Severity:      SeverityLow,
			Status:        StatusOpen,
			Title:         title,
			Summary:       fmt.Sprintf("Table exceeds 1 GiB (%d bytes). Review retention/compression.", t.SizeBytes),
			ObjectType:    "table",
			ObjectKey:     key,
			DatabaseName:  t.Database,
			SchemaName:    t.Schema,
			ObjectName:    t.Name,
			Evidence: map[string]any{
				"size_bytes": t.SizeBytes,
				"threshold":  oneGiB,
			},
			DedupKey: DedupKey("storage.large_table", key, title),
		}
		out = append(out, f)
	}
	return out, nil
}
