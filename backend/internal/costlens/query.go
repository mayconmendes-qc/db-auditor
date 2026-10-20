package costlens

import (
	"fmt"

	"github.com/osmendes/db-auditor/internal/digest"
)

const (
	// Query share / call thresholds (initial, calibrate).
	QueryShareLow    = 0.10
	QueryShareMedium = 0.25
	QueryMinCalls    = int64(1000)
)

// QueryCostInput is one fingerprint's window contribution.
type QueryCostInput struct {
	Fingerprint     string
	Database        string
	DeltaCalls      int64
	DeltaTotalMS    float64
	DeltaBlocksRead int64
	TotalTimeMS     float64
	TotalBlocksRead int64
}

// QueryWindowStatus describes whether query cost can be computed.
type QueryWindowStatus struct {
	OK     bool
	Reason string
}

// ValidateQueryWindow rejects incomplete edges and reset-without-subwindow (CA-03).
func ValidateQueryWindow(result digest.WindowResult) QueryWindowStatus {
	if !result.OK {
		reason := result.Reason
		if reason == "" {
			reason = ReasonInvalidWindow
		}
		if reason == "sem janela comparável: stats_reset sem subjanela contígua" {
			return QueryWindowStatus{OK: false, Reason: ReasonInvalidWindow}
		}
		return QueryWindowStatus{OK: false, Reason: reason}
	}
	return QueryWindowStatus{OK: true}
}

// EstimateQueryCost prices one fingerprint's share of compute and I/O (RF-05).
func EstimateQueryCost(in QueryCostInput, price *PriceVersion, windowOK QueryWindowStatus) CostResult {
	r := CostResult{
		Type:        "query",
		ObjectKey:   in.Fingerprint,
		Database:    in.Database,
		Fingerprint: in.Fingerprint,
		DeltaCalls:  in.DeltaCalls,
		Confidence:  "medium",
		Label:       ConfidenceEstimate,
		FindingType: "cost.query_expensive",
		NextStep:    "Revise o plano e índices candidatos; custo é participação na janela, não fatura.",
		ImpactBasis: map[string]any{
			"delta_calls":       in.DeltaCalls,
			"delta_total_ms":    in.DeltaTotalMS,
			"delta_blocks_read": in.DeltaBlocksRead,
			"period":            "mês (projeção da janela)",
		},
	}
	if !windowOK.OK {
		r.Reason = windowOK.Reason
		if r.Reason == "" {
			r.Reason = ReasonInvalidWindow
		}
		r.Label = r.Reason
		r.Confidence = "none"
		return r
	}
	var shareTime, shareIO float64
	if in.TotalTimeMS > 0 && in.DeltaTotalMS > 0 {
		shareTime = in.DeltaTotalMS / in.TotalTimeMS
	}
	if in.TotalBlocksRead > 0 && in.DeltaBlocksRead > 0 {
		shareIO = float64(in.DeltaBlocksRead) / float64(in.TotalBlocksRead)
	}
	r.ShareOfTime = shareTime
	r.ImpactBasis["share_of_time"] = shareTime
	r.ImpactBasis["share_of_io"] = shareIO

	if price == nil {
		r.Reason = ReasonNoPrice
		r.Label = ReasonNoPrice
		r.Confidence = "none"
		return r
	}

	var computeMonth, ioMonth float64
	if price.InstanceMonth != nil {
		computeMonth = *price.InstanceMonth * shareTime
	} else if price.ComputeVCPUHour != nil {
		// Rough: one vCPU full month ≈ 730 h; share of time ≈ share of that bill.
		computeMonth = (*price.ComputeVCPUHour) * 730 * shareTime
	}
	if price.IOPerGBRead != nil {
		// 8 KiB blocks → GB.
		gb := float64(in.DeltaBlocksRead) * 8192 / 1e9
		ioMonth = gb * (*price.IOPerGBRead)
	} else if price.IOPerMillion != nil && in.DeltaBlocksRead > 0 {
		ioMonth = (float64(in.DeltaBlocksRead) / 1e6) * (*price.IOPerMillion)
	}
	amount := RoundMoney(computeMonth + ioMonth)
	r.Priced = true
	r.MonthAmount = &amount
	r.Currency = price.Currency
	r.PriceVersion = price.Version
	r.PriceEffective = price.EffectiveFrom.UTC().Format("2006-01-02")
	r.Formula = fmt.Sprintf("share_tempo=%.1f%% × compute + share_io=%.1f%% × I/O → %s",
		shareTime*100, shareIO*100, MoneyLabel(amount, price.Currency))
	r.Label = MoneyLabel(amount, price.Currency)
	r.ImpactBasis["price_version"] = price.Version
	r.ImpactBasis["price_effective_from"] = r.PriceEffective
	r.ImpactBasis["currency"] = price.Currency
	r.ImpactBasis["formula"] = r.Formula
	r.ImpactBasis["month_amount"] = amount
	return r
}

// MeetsQueryExpensiveRule reports whether cost.query_expensive should fire.
func MeetsQueryExpensiveRule(share float64, calls int64) bool {
	return share >= QueryShareLow && calls >= QueryMinCalls
}

// QuerySeverity is low at ≥10% share, medium at ≥25%.
func QuerySeverity(share float64) string {
	if share >= QueryShareMedium {
		return "medium"
	}
	return "low"
}

// UnusedIndexThresholds (initial, calibrate).
const (
	UnusedIndexMinBytes     = int64(100 << 20) // 100 MiB
	UnusedIndexHighBytes    = int64(5 << 30)   // 5 GiB
	UnusedIndexMinWindowDay = 14
)
