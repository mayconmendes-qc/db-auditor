package analyzer

import (
	"context"
	"testing"
	"time"

	"github.com/osmendes/db-auditor/internal/costlens"
)

func TestCostUnusedIndexWasteFinding(t *testing.T) {
	now := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	prev := now.Add(-20 * 24 * time.Hour)
	price := &CostPriceFact{Version: 1, Currency: "BRL", StorageGBMonth: 0.5, EffectiveFrom: prev}
	facts := SnapshotFacts{
		EnvironmentID: "e", AuditRunID: "r", CostPrice: price,
		Indexes: []IndexFact{{
			Database: "db", Schema: "public", TableName: "t", IndexName: "idx",
			SizeBytes: 2e9, IdxScan: 0, CollectedAt: now,
		}},
		PreviousIndexes: []IndexFact{{
			Database: "db", Schema: "public", TableName: "t", IndexName: "idx",
			SizeBytes: 2e9, IdxScan: 0, CollectedAt: prev,
		}},
	}
	out, err := (CostAnalyzer{}).Analyze(context.Background(), facts)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range out {
		if f.FindingType == "cost.unused_index_waste" {
			found = true
			if f.Evidence["cost_label"] == nil {
				t.Fatalf("missing cost_label: %+v", f.Evidence)
			}
		}
	}
	if !found {
		t.Fatal("expected cost.unused_index_waste")
	}
}

func TestCostBloatRecoverableFinding(t *testing.T) {
	const threeGiB = int64(3 << 30)
	rowEstimate := int64(1_000_000)
	targetEst := float64(threeGiB) * 0.65
	avgWidth := int(targetEst/float64(rowEstimate)) - 24
	facts := SnapshotFacts{
		EnvironmentID: "e", AuditRunID: "r",
		Tables: []TableFact{{Database: "db", Schema: "public", Name: "t", SizeBytes: threeGiB, RowEstimate: rowEstimate}},
		ColumnStats: []ColumnStatFact{{
			Database: "db", Schema: "public", TableName: "t", ColumnName: "c",
			AverageWidth: avgWidth, NullFraction: 0,
		}},
	}
	out, err := (CostAnalyzer{}).Analyze(context.Background(), facts)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range out {
		if f.FindingType == "cost.bloat_recoverable" {
			if f.Evidence["label"] != costlens.ConfidenceEstimate {
				t.Fatalf("label: %+v", f.Evidence)
			}
			if f.Severity != SeverityMedium {
				t.Fatalf("severity %s", f.Severity)
			}
			return
		}
	}
	t.Fatal("expected cost.bloat_recoverable")
}

func TestCostQueryInvalidWindowNoFinding(t *testing.T) {
	facts := SnapshotFacts{
		EnvironmentID:   "e",
		QueryCostWindow: &QueryCostWindowFact{OK: false, Reason: costlens.ReasonInvalidWindow},
	}
	out, err := (CostAnalyzer{}).Analyze(context.Background(), facts)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range out {
		if f.FindingType == "cost.query_expensive" {
			t.Fatal("invalid window must not emit query_expensive")
		}
	}
}
