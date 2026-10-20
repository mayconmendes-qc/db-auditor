package analyzer

import (
	"context"
	"fmt"

	"github.com/osmendes/db-auditor/internal/costlens"
)

// CostAnalyzer emits cost.* findings from snapshot facts (spec 02).
// Monetary amounts appear only when CostPrice is set on SnapshotFacts.
type CostAnalyzer struct{}

func (CostAnalyzer) Name() string { return "cost" }

func priceFromFacts(f *CostPriceFact) *costlens.PriceVersion {
	if f == nil {
		return nil
	}
	return &costlens.PriceVersion{
		Version: f.Version, Currency: f.Currency, StorageGBMonth: f.StorageGBMonth,
		IOPerMillion: f.IOPerMillion, IOPerGBRead: f.IOPerGBRead,
		ComputeVCPUHour: f.ComputeVCPUHour, InstanceMonth: f.InstanceMonth,
		EffectiveFrom: f.EffectiveFrom,
	}
}

func (CostAnalyzer) Analyze(_ context.Context, facts SnapshotFacts) ([]Finding, error) {
	out := make([]Finding, 0)
	price := priceFromFacts(facts.CostPrice)

	prev := map[string]IndexFact{}
	for _, idx := range facts.PreviousIndexes {
		prev[indexKey(idx)] = idx
	}
	for _, idx := range facts.Indexes {
		if idx.IsPrimary || idx.IsUnique {
			continue
		}
		if idx.SizeBytes < costlens.UnusedIndexMinBytes {
			continue
		}
		earlier, ok := prev[indexKey(idx)]
		if !ok || idx.IdxScan != earlier.IdxScan {
			continue
		}
		if idx.StatsReset != nil && !earlier.CollectedAt.IsZero() && idx.StatsReset.After(earlier.CollectedAt) {
			continue
		}
		windowDays := 0.0
		if !earlier.CollectedAt.IsZero() && idx.CollectedAt.After(earlier.CollectedAt) {
			windowDays = idx.CollectedAt.Sub(earlier.CollectedAt).Hours() / 24
		}
		if windowDays < float64(costlens.UnusedIndexMinWindowDay) {
			continue
		}
		key := indexKey(idx)
		sev := SeverityMedium
		if idx.SizeBytes >= costlens.UnusedIndexHighBytes {
			sev = SeverityHigh
		}
		est := costlens.EstimateUnusedIndexWaste(idx.SizeBytes, key, idx.Database, idx.Schema, idx.IndexName, "", price)
		title := fmt.Sprintf("Desperdício de índice sem uso: %s", key)
		evidence := map[string]any{
			"size_bytes":   idx.SizeBytes,
			"window_days":  windowDays,
			"note":         UnusedIndexNote,
			"impact_basis": est.ImpactBasis,
			"cost_label":   est.Label,
		}
		if !est.Priced {
			evidence["cost_reason"] = est.Reason
		}
		out = append(out, Finding{
			EnvironmentID: facts.EnvironmentID, AuditRunID: facts.AuditRunID,
			FindingType: "cost.unused_index_waste", Severity: sev, Status: StatusOpen,
			Title: title, Summary: "Índice sem uso observado na janela, valorado por tamanho × preço de armazenamento (estimado/mês).",
			ObjectType: "index", ObjectKey: key, DatabaseName: idx.Database, SchemaName: idx.Schema, ObjectName: idx.IndexName,
			Evidence: evidence, DedupKey: DedupKey("cost.unused_index_waste", key, title),
		})
	}

	colsByTable := map[string][]costlens.ColumnWidthStat{}
	for _, c := range facts.ColumnStats {
		key := c.Database + "." + c.Schema + "." + c.TableName
		colsByTable[key] = append(colsByTable[key], costlens.ColumnWidthStat{AverageWidth: c.AverageWidth, NullFraction: c.NullFraction})
	}
	for _, t := range facts.Tables {
		key := t.Database + "." + t.Schema + "." + t.Name
		est := costlens.EstimateBloat(t.SizeBytes, t.RowEstimate, colsByTable[key])
		if !costlens.MeetsBloatRule(est) {
			continue
		}
		cost := costlens.EstimateBloatCost(est, key, t.Database, t.Schema, t.Name, price)
		sev := SeverityMedium
		if costlens.BloatSeverity(est) == "high" {
			sev = SeverityHigh
		}
		title := fmt.Sprintf("Bloat recuperável estimado: %s", key)
		evidence := map[string]any{
			"bloat_ratio":  est.BloatRatio,
			"bloat_bytes":  est.BloatBytes,
			"table_bytes":  est.TableBytes,
			"label":        costlens.ConfidenceEstimate,
			"impact_basis": cost.ImpactBasis,
			"cost_label":   cost.Label,
		}
		if !cost.Priced {
			evidence["cost_reason"] = cost.Reason
		}
		out = append(out, Finding{
			EnvironmentID: facts.EnvironmentID, AuditRunID: facts.AuditRunID,
			FindingType: "cost.bloat_recoverable", Severity: sev, Status: StatusOpen,
			Title: title, Summary: "Bloat estimado estatisticamente (não medido). Confiança no máximo média.",
			ObjectType: "table", ObjectKey: key, DatabaseName: t.Database, SchemaName: t.Schema, ObjectName: t.Name,
			Evidence: evidence, DedupKey: DedupKey("cost.bloat_recoverable", key, title),
		})
	}

	if facts.QueryCostWindow != nil {
		win := *facts.QueryCostWindow
		if !win.OK {
			return out, nil
		}
		var totalBlocks int64
		for _, d := range win.Deltas {
			if d.DeltaBlocksRead > 0 {
				totalBlocks += d.DeltaBlocksRead
			}
		}
		status := costlens.QueryWindowStatus{OK: true}
		for _, d := range win.Deltas {
			share := 0.0
			if win.TotalTimeMS > 0 {
				share = d.DeltaTotalMS / win.TotalTimeMS
			}
			if !costlens.MeetsQueryExpensiveRule(share, d.DeltaCalls) {
				continue
			}
			in := costlens.QueryCostInput{
				Fingerprint: d.Fingerprint, Database: d.DatabaseName,
				DeltaCalls: d.DeltaCalls, DeltaTotalMS: d.DeltaTotalMS, DeltaBlocksRead: d.DeltaBlocksRead,
				TotalTimeMS: win.TotalTimeMS, TotalBlocksRead: totalBlocks,
			}
			cost := costlens.EstimateQueryCost(in, price, status)
			sev := SeverityLow
			if costlens.QuerySeverity(share) == "medium" {
				sev = SeverityMedium
			}
			title := fmt.Sprintf("Consulta cara na janela: %s", d.Fingerprint)
			evidence := map[string]any{
				"fingerprint":   d.Fingerprint,
				"share_of_time": share,
				"delta_calls":   d.DeltaCalls,
				"impact_basis":  cost.ImpactBasis,
				"cost_label":    cost.Label,
			}
			if !cost.Priced {
				evidence["cost_reason"] = cost.Reason
			}
			out = append(out, Finding{
				EnvironmentID: facts.EnvironmentID, AuditRunID: facts.AuditRunID,
				FindingType: "cost.query_expensive", Severity: sev, Status: StatusOpen,
				Title: title, Summary: "Fingerprint com participação elevada no tempo total da janela entre coletas completas.",
				ObjectType: "fingerprint", ObjectKey: d.Fingerprint, DatabaseName: d.DatabaseName, ObjectName: d.Fingerprint,
				Evidence: evidence, DedupKey: DedupKey("cost.query_expensive", d.Fingerprint, title),
			})
		}
	}
	return out, nil
}
