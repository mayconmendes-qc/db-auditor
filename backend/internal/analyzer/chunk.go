package analyzer

import (
	"context"
	"fmt"
	"math"
)

const highChunkThreshold = 500

// ChunkAnalyzer flags hypertables with many chunks or abnormal size distribution.
type ChunkAnalyzer struct{}

func (ChunkAnalyzer) Name() string { return "chunk" }

func (ChunkAnalyzer) Analyze(_ context.Context, facts SnapshotFacts) ([]Finding, error) {
	out := make([]Finding, 0)
	for _, ht := range facts.Hypertables {
		if ht.NumChunks < highChunkThreshold {
			continue
		}
		key := fmt.Sprintf("%s.%s.%s", ht.Database, ht.Schema, ht.Name)
		title := fmt.Sprintf("High chunk count: %s", key)
		f := Finding{
			EnvironmentID: facts.EnvironmentID,
			AuditRunID:    facts.AuditRunID,
			FindingType:   "chunk.high_count",
			Severity:      SeverityMedium,
			Status:        StatusOpen,
			Title:         title,
			Summary:       fmt.Sprintf("Hypertable has %d chunks (threshold %d). Consider interval review.", ht.NumChunks, highChunkThreshold),
			ObjectType:    "hypertable",
			ObjectKey:     key,
			DatabaseName:  ht.Database,
			SchemaName:    ht.Schema,
			ObjectName:    ht.Name,
			Evidence: map[string]any{
				"num_chunks": ht.NumChunks,
				"threshold":  highChunkThreshold,
				"size_bytes": ht.SizeBytes,
			},
			DedupKey: DedupKey("chunk.high_count", key, title),
		}
		out = append(out, f)
	}
	if facts.ChunksTruncated {
		out = append(out, Finding{
			EnvironmentID: facts.EnvironmentID,
			AuditRunID:    facts.AuditRunID,
			FindingType:   "chunk.inventory_truncated",
			Severity:      SeverityMedium,
			Status:        StatusOpen,
			Title:         "Chunk inventory truncated",
			Summary:       "The chunk list hit the collection cap. Coverage is incomplete and size skew uses the aggregate, not every chunk row.",
			ObjectType:    "environment",
			ObjectKey:     facts.EnvironmentID,
			DedupKey:      DedupKey("chunk.inventory_truncated", facts.EnvironmentID, "Chunk inventory truncated"),
		})
	}
	if len(facts.ChunkStats) > 0 {
		for _, stat := range facts.ChunkStats {
			if stat.Count < 3 || stat.SumBytes <= 0 {
				continue
			}
			mean := float64(stat.SumBytes) / float64(stat.Count)
			ratio := float64(stat.MaxBytes) / mean
			if ratio < 5 {
				continue
			}
			key := fmt.Sprintf("%s.%s.%s", stat.Database, stat.Schema, stat.HypertableName)
			title := fmt.Sprintf("Abnormal chunk size distribution: %s", key)
			out = append(out, Finding{
				EnvironmentID: facts.EnvironmentID,
				AuditRunID:    facts.AuditRunID,
				FindingType:   "chunk.size_skew",
				Severity:      SeverityLow,
				Status:        StatusOpen,
				Title:         title,
				Summary:       fmt.Sprintf("Largest chunk is %.1fx the mean size across %d chunks.", ratio, stat.Count),
				ObjectType:    "hypertable",
				ObjectKey:     key,
				DatabaseName:  stat.Database,
				SchemaName:    stat.Schema,
				ObjectName:    stat.HypertableName,
				Evidence: map[string]any{
					"chunk_count": stat.Count,
					"max_bytes":   stat.MaxBytes,
					"mean_bytes":  math.Round(mean),
					"ratio":       math.Round(ratio*10) / 10,
					"aggregated":  true,
				},
				DedupKey: DedupKey("chunk.size_skew", key, title),
			})
		}
		return out, nil
	}
	byHT := map[string][]ChunkFact{}
	for _, c := range facts.Chunks {
		k := fmt.Sprintf("%s.%s.%s", c.Database, c.Schema, c.HypertableName)
		byHT[k] = append(byHT[k], c)
	}
	for key, chunks := range byHT {
		if len(chunks) < 3 {
			continue
		}
		var sum, max int64
		for _, c := range chunks {
			sum += c.SizeBytes
			if c.SizeBytes > max {
				max = c.SizeBytes
			}
		}
		mean := float64(sum) / float64(len(chunks))
		if mean <= 0 {
			continue
		}
		ratio := float64(max) / mean
		if ratio < 5 {
			continue
		}
		title := fmt.Sprintf("Abnormal chunk size distribution: %s", key)
		f := Finding{
			EnvironmentID: facts.EnvironmentID,
			AuditRunID:    facts.AuditRunID,
			FindingType:   "chunk.size_skew",
			Severity:      SeverityLow,
			Status:        StatusOpen,
			Title:         title,
			Summary:       fmt.Sprintf("Largest chunk is %.1fx the mean size across %d chunks.", ratio, len(chunks)),
			ObjectType:    "hypertable",
			ObjectKey:     key,
			DatabaseName:  chunks[0].Database,
			SchemaName:    chunks[0].Schema,
			ObjectName:    chunks[0].HypertableName,
			Evidence: map[string]any{
				"chunk_count": len(chunks),
				"max_bytes":   max,
				"mean_bytes":  math.Round(mean),
				"ratio":       math.Round(ratio*10) / 10,
			},
			DedupKey: DedupKey("chunk.size_skew", key, title),
		}
		out = append(out, f)
	}
	return out, nil
}
