package costlens

import "sort"

// Ranking holds the top waste items and optional totals.
type Ranking struct {
	Items        []CostResult  `json:"items"`
	Reason       string        `json:"reason,omitempty"`
	Priced       bool          `json:"priced"`
	Currency     string        `json:"currency,omitempty"`
	PriceVersion int           `json:"price_version,omitempty"`
	Totals       RankingTotals `json:"totals"`
}

// RankingTotals aggregates monthly amounts by type.
type RankingTotals struct {
	Storage     float64 `json:"storage_month"`
	UnusedIndex float64 `json:"unused_index_month"`
	Bloat       float64 `json:"bloat_month"`
	Query       float64 `json:"query_month"`
}

// BuildRanking merges cost items, sorts by month amount (unpriced last by size), keeps top N.
func BuildRanking(items []CostResult, topN int, price *PriceVersion, emptyReason string) Ranking {
	if topN <= 0 {
		topN = 10
	}
	out := Ranking{Items: []CostResult{}, Reason: emptyReason}
	if price != nil {
		out.Priced = true
		out.Currency = price.Currency
		out.PriceVersion = price.Version
	}
	if len(items) == 0 {
		if out.Reason == "" {
			out.Reason = "sem coleta completa"
		}
		return out
	}
	sorted := append([]CostResult(nil), items...)
	sort.SliceStable(sorted, func(i, j int) bool {
		ai, aj := sorted[i].MonthAmount, sorted[j].MonthAmount
		if ai != nil && aj != nil {
			if *ai != *aj {
				return *ai > *aj
			}
		} else if ai != nil {
			return true
		} else if aj != nil {
			return false
		}
		return sorted[i].SizeBytes > sorted[j].SizeBytes
	})
	if len(sorted) > topN {
		sorted = sorted[:topN]
	}
	out.Items = sorted
	out.Reason = ""
	for _, it := range items {
		if it.MonthAmount == nil {
			continue
		}
		switch it.Type {
		case "storage":
			out.Totals.Storage += *it.MonthAmount
		case "unused_index":
			out.Totals.UnusedIndex += *it.MonthAmount
		case "bloat":
			out.Totals.Bloat += *it.MonthAmount
		case "query":
			out.Totals.Query += *it.MonthAmount
		}
	}
	out.Totals.Storage = RoundMoney(out.Totals.Storage)
	out.Totals.UnusedIndex = RoundMoney(out.Totals.UnusedIndex)
	out.Totals.Bloat = RoundMoney(out.Totals.Bloat)
	out.Totals.Query = RoundMoney(out.Totals.Query)
	return out
}

// SeriesPoint is one complete-run cost total (gaps are absent points).
type SeriesPoint struct {
	AuditRunID       string  `json:"audit_run_id"`
	At               string  `json:"at"`
	Completeness     string  `json:"completeness"` // complete | gap
	Currency         string  `json:"currency,omitempty"`
	StorageMonth     float64 `json:"storage_month"`
	UnusedIndexMonth float64 `json:"unused_index_month"`
	BloatMonth       float64 `json:"bloat_month"`
	QueryMonth       float64 `json:"query_month"`
	Priced           bool    `json:"priced"`
	Confidence       string  `json:"confidence,omitempty"`
	Reason           string  `json:"reason,omitempty"`
}
