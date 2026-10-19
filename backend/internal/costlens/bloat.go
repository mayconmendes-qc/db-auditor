package costlens

import "fmt"

// ColumnWidthStat is one column's planner estimates (pg_stats).
type ColumnWidthStat struct {
	AverageWidth int
	NullFraction float64
}

// BloatEstimate is the statistical recoverable-space estimate (never "measured").
type BloatEstimate struct {
	EstimatedDataBytes int64
	TableBytes         int64
	BloatBytes         int64
	BloatRatio         float64
	Label              string // always "estimativa"
	Confidence         string // at most "medium"
}

const (
	// Tuple header + item pointer overhead approximation (bytes per row).
	rowOverheadBytes = 24
	pageHeaderApprox = 24
	// Thresholds (initial, calibrate): RF-04 / cost.bloat_recoverable.
	BloatRatioThreshold = 0.30
	BloatBytesThreshold = int64(1 << 30) // 1 GiB
	BloatHighBytes      = int64(20 << 30)
)

// EstimateBloat computes statistical bloat without reading row data (ADR-0001 D3).
// estimated_data = rows * (overhead + Σ avg_width*(1-null_frac)); bloat = max(0, table - estimated).
func EstimateBloat(tableBytes, rowEstimate int64, cols []ColumnWidthStat) BloatEstimate {
	out := BloatEstimate{
		TableBytes: tableBytes,
		Label:      ConfidenceEstimate,
		Confidence: "medium",
	}
	if tableBytes <= 0 || rowEstimate <= 0 || len(cols) == 0 {
		out.Confidence = "low"
		return out
	}
	var width float64
	for _, c := range cols {
		nf := c.NullFraction
		if nf < 0 {
			nf = 0
		}
		if nf > 1 {
			nf = 1
		}
		w := float64(c.AverageWidth)
		if w < 0 {
			w = 0
		}
		width += w * (1 - nf)
	}
	perRow := float64(rowOverheadBytes) + width
	estimated := int64(float64(rowEstimate)*perRow) + pageHeaderApprox
	if estimated < 0 {
		estimated = 0
	}
	out.EstimatedDataBytes = estimated
	if tableBytes > estimated {
		out.BloatBytes = tableBytes - estimated
	}
	if tableBytes > 0 {
		out.BloatRatio = float64(out.BloatBytes) / float64(tableBytes)
	}
	return out
}

// MeetsBloatRule reports whether cost.bloat_recoverable should fire.
func MeetsBloatRule(e BloatEstimate) bool {
	return e.BloatRatio >= BloatRatioThreshold && e.BloatBytes >= BloatBytesThreshold
}

// BloatSeverity is medium, or high when recoverable space ≥ 20 GiB.
func BloatSeverity(e BloatEstimate) string {
	if e.BloatBytes >= BloatHighBytes {
		return "high"
	}
	return "medium"
}

// EstimateBloatCost prices recoverable bloat bytes as storage waste.
func EstimateBloatCost(e BloatEstimate, objectKey, database, schema, name string, price *PriceVersion) CostResult {
	r := CostResult{
		Type:       "bloat",
		ObjectKey:  objectKey,
		Database:   database,
		Schema:     schema,
		Object:     name,
		SizeBytes:  e.BloatBytes,
		Confidence: e.Confidence,
		Label:      ConfidenceEstimate,
		NextStep:   "Revise VACUUM/FILLFACTOR e retenção; bloat é estimativa estatística, não medição física.",
		ImpactBasis: map[string]any{
			"bloat_bytes":          e.BloatBytes,
			"bloat_ratio":          e.BloatRatio,
			"table_bytes":          e.TableBytes,
			"estimated_data_bytes": e.EstimatedDataBytes,
			"label":                ConfidenceEstimate,
			"period":               "mês",
		},
		FindingType: "cost.bloat_recoverable",
	}
	if price == nil {
		r.Reason = ReasonNoPrice
		r.Label = ReasonNoPrice
		r.Confidence = "none"
		return r
	}
	amount := StorageMonthlyCost(e.BloatBytes, price.StorageGBMonth)
	r.Priced = true
	r.MonthAmount = &amount
	r.Currency = price.Currency
	r.PriceVersion = price.Version
	r.PriceEffective = price.EffectiveFrom.UTC().Format("2006-01-02")
	r.Formula = fmt.Sprintf("bloat estimado %.1f%% (%.3f GB) × %s %.4f/GB-mês = %s",
		e.BloatRatio*100, BytesToGB(e.BloatBytes), price.Currency, price.StorageGBMonth, MoneyLabel(amount, price.Currency))
	r.Label = MoneyLabel(amount, price.Currency) + " (" + ConfidenceEstimate + ")"
	r.ImpactBasis["storage_gb_month"] = price.StorageGBMonth
	r.ImpactBasis["price_version"] = price.Version
	r.ImpactBasis["price_effective_from"] = r.PriceEffective
	r.ImpactBasis["currency"] = price.Currency
	r.ImpactBasis["formula"] = r.Formula
	r.ImpactBasis["month_amount"] = amount
	return r
}
