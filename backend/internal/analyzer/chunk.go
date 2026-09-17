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
	// Size skew: for each hypertable, if max chunk >> mean, flag.
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
