package costlens

import (
	"testing"
	"time"

	"github.com/osmendes/db-auditor/internal/digest"
)

func TestUnusedIndexWasteCost(t *testing.T) {
	// CA-01: 2 GB × R$ 0,50/GB-mês = R$ 1,00/mês
	price := &PriceVersion{
		Version: 1, Currency: CurrencyBRL, StorageGBMonth: 0.50,
		EffectiveFrom: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	const twoGB = int64(2e9)
	got := EstimateUnusedIndexWaste(twoGB, "db.public.idx", "db", "public", "idx", "f1", price)
	if !got.Priced || got.MonthAmount == nil || *got.MonthAmount != 1.00 {
		t.Fatalf("want R$ 1.00, got %+v", got)
	}
	if got.Formula == "" || got.PriceEffective == "" {
		t.Fatalf("missing formula/vigência: %+v", got)
	}
	none := EstimateUnusedIndexWaste(twoGB, "db.public.idx", "db", "public", "idx", "f1", nil)
	if none.Priced || none.Reason != ReasonNoPrice {
		t.Fatalf("without price: %+v", none)
	}
}

func TestBloatEstimateRule(t *testing.T) {
	// CA-04: 35% bloat on ≥ 3 GiB → medium, labeled estimativa
	const threeGiB = int64(3 << 30)
	// Force ~35% bloat: estimated data = 65% of table.
	rowEstimate := int64(1_000_000)
	targetEst := float64(threeGiB) * 0.65
	avgWidth := int(targetEst/float64(rowEstimate)) - rowOverheadBytes
	if avgWidth < 1 {
		t.Fatal("fixture width")
	}
	e := EstimateBloat(threeGiB, rowEstimate, []ColumnWidthStat{{AverageWidth: avgWidth, NullFraction: 0}})
	if e.BloatRatio < 0.30 || e.Label != ConfidenceEstimate {
		t.Fatalf("bloat estimate: %+v", e)
	}
	if !MeetsBloatRule(e) {
		t.Fatalf("should meet rule: %+v", e)
	}
	if BloatSeverity(e) != "medium" {
		t.Fatalf("severity: %s", BloatSeverity(e))
	}
	price := &PriceVersion{Version: 1, Currency: CurrencyBRL, StorageGBMonth: 0.5, EffectiveFrom: time.Now().UTC()}
	c := EstimateBloatCost(e, "db.public.t", "db", "public", "t", price)
	if c.Label == "" || c.Confidence != "medium" {
		t.Fatalf("cost label: %+v", c)
	}
}

func TestQueryCostInvalidWindowOnStatsReset(t *testing.T) {
	// CA-03
	st := ValidateQueryWindow(digest.WindowResult{
		OK: false, Reason: "sem janela comparável: stats_reset sem subjanela contígua",
	})
	if st.OK || st.Reason != ReasonInvalidWindow {
		t.Fatalf("status: %+v", st)
	}
	got := EstimateQueryCost(QueryCostInput{Fingerprint: "abc", DeltaCalls: 2000, DeltaTotalMS: 5000, TotalTimeMS: 10000}, nil, st)
	if got.Reason != ReasonInvalidWindow || got.Priced {
		t.Fatalf("query cost: %+v", got)
	}
}

func TestPriceValidation(t *testing.T) {
	eff := time.Now().UTC()
	if err := ValidatePriceInput(PriceInput{Currency: "EUR", StorageGBMonth: 1, EffectiveFrom: eff}, ""); err == nil {
		t.Fatal("EUR should fail")
	}
	if err := ValidatePriceInput(PriceInput{Currency: "BRL", StorageGBMonth: -1, EffectiveFrom: eff}, ""); err == nil {
		t.Fatal("negative should fail")
	}
	if err := ValidatePriceInput(PriceInput{Currency: "USD", StorageGBMonth: 1, EffectiveFrom: eff}, "BRL"); err == nil {
		t.Fatal("currency lock should fail")
	}
	if err := ValidatePriceInput(PriceInput{Currency: "BRL", StorageGBMonth: 0.5, EffectiveFrom: eff, ModelOrigin: "self_hosted"}, "BRL"); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePriceInput(PriceInput{Currency: "BRL", StorageGBMonth: 0.5}, ""); err == nil {
		t.Fatal("effective_from required")
	}
}

func TestQueryExpensiveThresholds(t *testing.T) {
	if MeetsQueryExpensiveRule(0.09, 2000) {
		t.Fatal("below share")
	}
	if MeetsQueryExpensiveRule(0.10, 999) {
		t.Fatal("below calls")
	}
	if !MeetsQueryExpensiveRule(0.10, 1000) {
		t.Fatal("should meet")
	}
	if QuerySeverity(0.25) != "medium" || QuerySeverity(0.10) != "low" {
		t.Fatal("severity")
	}
}

func TestBuildRankingTop10(t *testing.T) {
	price := &PriceVersion{Version: 2, Currency: CurrencyUSD, StorageGBMonth: 1, EffectiveFrom: time.Now().UTC()}
	var items []CostResult
	for i := 0; i < 20; i++ {
		amt := float64(20 - i)
		items = append(items, CostResult{Type: "unused_index", ObjectKey: "k", MonthAmount: &amt, Priced: true, SizeBytes: int64(i)})
	}
	r := BuildRanking(items, 10, price, "")
	if len(r.Items) != 10 || !r.Priced || r.Currency != CurrencyUSD {
		t.Fatalf("ranking: %+v", r)
	}
}

func TestTemplates(t *testing.T) {
	ts := Templates()
	if len(ts) != 3 {
		t.Fatalf("want 3 templates, got %d", len(ts))
	}
}
