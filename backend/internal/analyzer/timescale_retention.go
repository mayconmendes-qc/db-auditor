package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	retentionMissingMinBytes     = 10 * 1024 * 1024 * 1024 // 10 GB — valor inicial, calibrar
	compressionMissingMinBytes   = 5 * 1024 * 1024 * 1024  // 5 GB
	compressionMissingAgeDays    = 30
	compressionMissingSizeShare  = 0.50
	compressionRatioLowThreshold = 2.0
	compressionRatioLowMinChunks = 5
	retentionOldChunkDays        = 365
)

// TimescaleRetentionAnalyzer emits timescale.retention_* and timescale.compression_* (spec 09).
type TimescaleRetentionAnalyzer struct{}

func (TimescaleRetentionAnalyzer) Name() string { return "timescale_retention" }

func (TimescaleRetentionAnalyzer) Analyze(_ context.Context, facts SnapshotFacts) ([]Finding, error) {
	out := make([]Finding, 0)
	hasRetention := map[string]bool{}
	hasCompression := map[string]bool{}
	retentionDays := map[string]int{}
	for _, p := range facts.Policies {
		ht := fmt.Sprintf("%s.%s.%s", p.Database, p.HypertableSchema, p.HypertableName)
		switch strings.ToLower(p.PolicyType) {
		case "retention":
			hasRetention[ht] = true
			if d := intervalDays(policyInterval(p.Config)); d > 0 {
				retentionDays[ht] = d
			}
		case "compression", "columnstore":
			hasCompression[ht] = true
		}
	}

	caggBySource := map[string][]CAGGFact{}
	for _, c := range facts.CAGGs {
		srcSchema, srcName := c.Schema, c.ViewName
		if c.SourceSchema != "" && c.SourceName != "" {
			srcSchema, srcName = c.SourceSchema, c.SourceName
		}
		key := fmt.Sprintf("%s.%s.%s", c.Database, srcSchema, srcName)
		caggBySource[key] = append(caggBySource[key], c)
	}

	for _, ht := range facts.Hypertables {
		key := fmt.Sprintf("%s.%s.%s", ht.Database, ht.Schema, ht.Name)
		if ht.SizeBytes >= retentionMissingMinBytes && !hasRetention[key] && ht.ChunksOlderThanDays >= retentionOldChunkDays {
			title := fmt.Sprintf("Retenção ausente em hypertable grande: %s", key)
			out = append(out, Finding{
				EnvironmentID: facts.EnvironmentID,
				AuditRunID:    facts.AuditRunID,
				FindingType:   "timescale.retention_missing",
				Severity:      SeverityLow,
				Status:        StatusOpen,
				Title:         title,
				Summary:       "Hypertable ≥ 10 GB sem política de retenção e com chunks com mais de 365 dias. Retenção exige aprovação de negócio/compliance e backup restaurável; nenhuma exclusão automática.",
				ObjectType:    "hypertable",
				ObjectKey:     key,
				DatabaseName:  ht.Database,
				SchemaName:    ht.Schema,
				ObjectName:    ht.Name,
				Evidence: map[string]any{
					"size_bytes":             ht.SizeBytes,
					"chunks_older_than_days": ht.ChunksOlderThanDays,
					"threshold_bytes":        retentionMissingMinBytes,
					"hypothesis":             "Política de retenção pode reduzir armazenamento; confirme requisitos de compliance antes de qualquer exclusão.",
				},
				DedupKey: DedupKey("timescale.retention_missing", key, title),
			})
		}

		uncompressedOldShare := 0.0
		if ht.SizeBytes > 0 && ht.UncompressedOldBytes > 0 {
			uncompressedOldShare = float64(ht.UncompressedOldBytes) / float64(ht.SizeBytes)
		}
		noCompression := !hasCompression[key] || !ht.CompressionEnabled
		if ht.SizeBytes >= compressionMissingMinBytes && noCompression && uncompressedOldShare >= compressionMissingSizeShare {
			title := fmt.Sprintf("Compressão ausente em hypertable grande: %s", key)
			out = append(out, Finding{
				EnvironmentID: facts.EnvironmentID,
				AuditRunID:    facts.AuditRunID,
				FindingType:   "timescale.compression_missing",
				Severity:      SeverityMedium,
				Status:        StatusOpen,
				Title:         title,
				Summary:       "Hypertable ≥ 5 GB sem política de compressão e com ≥ 50% do tamanho em chunks não comprimidos com mais de 30 dias.",
				ObjectType:    "hypertable",
				ObjectKey:     key,
				DatabaseName:  ht.Database,
				SchemaName:    ht.Schema,
				ObjectName:    ht.Name,
				Evidence: map[string]any{
					"size_bytes":             ht.SizeBytes,
					"uncompressed_old_bytes": ht.UncompressedOldBytes,
					"uncompressed_old_share": math.Round(uncompressedOldShare*1000) / 1000,
					"compression_enabled":    ht.CompressionEnabled,
					"has_compression_policy": hasCompression[key],
					"hypothesis":             "Compressão após N dias pode reduzir armazenamento; valide a taxa observada antes de aplicar.",
				},
				DedupKey: DedupKey("timescale.compression_missing", key, title),
			})
		}

		if ht.CompressedChunkCount >= compressionRatioLowMinChunks && ht.ObservedCompressionRatio > 0 &&
			ht.ObservedCompressionRatio < compressionRatioLowThreshold {
			title := fmt.Sprintf("Taxa de compressão baixa: %s", key)
			out = append(out, Finding{
				EnvironmentID: facts.EnvironmentID,
				AuditRunID:    facts.AuditRunID,
				FindingType:   "timescale.compression_ratio_low",
				Severity:      SeverityLow,
				Status:        StatusOpen,
				Title:         title,
				Summary:       fmt.Sprintf("Taxa observada %.2f× em %d chunks comprimidos (limiar inicial 2×).", ht.ObservedCompressionRatio, ht.CompressedChunkCount),
				ObjectType:    "hypertable",
				ObjectKey:     key,
				DatabaseName:  ht.Database,
				SchemaName:    ht.Schema,
				ObjectName:    ht.Name,
				Evidence: map[string]any{
					"observed_ratio":         ht.ObservedCompressionRatio,
					"compressed_chunk_count": ht.CompressedChunkCount,
					"threshold":              compressionRatioLowThreshold,
				},
				DedupKey: DedupKey("timescale.compression_ratio_low", key, title),
			})
		}

		// Vigente retention vs CAGG refresh window (also created on scenario save — ADR-0001 D9).
		if days, ok := retentionDays[key]; ok && days > 0 {
			for _, cagg := range caggBySource[key] {
				window := cagg.RefreshWindowDays
				if window == 0 && cagg.HasRefreshPolicy {
					window = intervalDays(cagg.Lag) // fallback unused usually
				}
				if cagg.HasRefreshPolicy && cagg.RefreshWindowDays > 0 && days < cagg.RefreshWindowDays {
					title := fmt.Sprintf("Retenção conflita com CAGG %s.%s", cagg.Schema, cagg.ViewName)
					out = append(out, Finding{
						EnvironmentID: facts.EnvironmentID,
						AuditRunID:    facts.AuditRunID,
						FindingType:   "timescale.retention_cagg_conflict",
						Severity:      SeverityHigh,
						Status:        StatusOpen,
						Title:         title,
						Summary:       fmt.Sprintf("Retenção vigente de %d dias é menor que a janela de refresh de %d dias do CAGG dependente.", days, cagg.RefreshWindowDays),
						ObjectType:    "hypertable",
						ObjectKey:     key,
						DatabaseName:  ht.Database,
						SchemaName:    ht.Schema,
						ObjectName:    ht.Name,
						Evidence: map[string]any{
							"retention_days":      days,
							"cagg":                fmt.Sprintf("%s.%s", cagg.Schema, cagg.ViewName),
							"refresh_window_days": cagg.RefreshWindowDays,
						},
						DedupKey: DedupKey("timescale.retention_cagg_conflict", key+":"+cagg.ViewName, title),
					})
				}
			}
		}
	}
	return out, nil
}

// ConflictFindingForSavedScenario builds the CA-04 finding at scenario save time (ADR-0001 D9).
func ConflictFindingForSavedScenario(env, run, database, schema, hypertable, caggView string, retentionDays, refreshWindowDays int) Finding {
	key := fmt.Sprintf("%s.%s.%s", database, schema, hypertable)
	title := fmt.Sprintf("Cenário de retenção conflita com CAGG %s", caggView)
	return Finding{
		EnvironmentID: env,
		AuditRunID:    run,
		FindingType:   "timescale.retention_cagg_conflict",
		Severity:      SeverityHigh,
		Status:        StatusOpen,
		Title:         title,
		Summary:       fmt.Sprintf("Cenário salvo com retenção de %d dias é menor que a janela de refresh de %d dias do CAGG dependente.", retentionDays, refreshWindowDays),
		ObjectType:    "hypertable",
		ObjectKey:     key,
		DatabaseName:  database,
		SchemaName:    schema,
		ObjectName:    hypertable,
		Evidence: map[string]any{
			"retention_days":      retentionDays,
			"cagg":                caggView,
			"refresh_window_days": refreshWindowDays,
			"source":              "saved_scenario",
		},
		DedupKey: DedupKey("timescale.retention_cagg_conflict", key+":scenario:"+caggView, title),
	}
}

func intervalDays(v string) int {
	v = strings.TrimSpace(strings.ToLower(v))
	if v == "" {
		return 0
	}
	fields := strings.Fields(v)
	if len(fields) >= 2 {
		var n float64
		if _, err := fmt.Sscanf(fields[0], "%f", &n); err == nil {
			unit := fields[1]
			switch {
			case strings.HasPrefix(unit, "day"):
				return int(n)
			case strings.HasPrefix(unit, "hour"):
				return int(n / 24)
			case strings.HasPrefix(unit, "week"):
				return int(n * 7)
			}
		}
	}
	if d := parseInterval(v); d > 0 {
		return int(d / (24 * time.Hour))
	}
	return 0
}

func policyInterval(config string) string {
	if config == "" {
		return ""
	}
	var raw map[string]any
	if json.Unmarshal([]byte(config), &raw) != nil {
		return ""
	}
	for _, key := range []string{"drop_after", "compress_after"} {
		if v, ok := raw[key].(string); ok {
			return v
		}
	}
	return ""
}
