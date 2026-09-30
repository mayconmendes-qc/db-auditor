package analyzer

import (
	"context"
	"testing"
	"time"
)

// TestPipelineCollectToFindings exercises analyzers with deterministic fixtures
// (T-267): no external database required.
func TestPipelineCollectToFindings(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	reset := now.Add(-60 * 24 * time.Hour) // beyond minimumIndexObservation

	facts := SnapshotFacts{
		EnvironmentID: "env-1",
		AuditRunID:    "run-1",
		Tables: []TableFact{
			{Database: "app", Schema: "public", Name: "events", SizeBytes: 9_000_000_000, CollectedAt: now},
		},
		Indexes: []IndexFact{
			{
				Database: "app", Schema: "public", TableName: "orders", IndexName: "idx_a",
				Definition: "CREATE INDEX idx_a ON public.orders USING btree (customer_id)",
				IdxScan: 0, SizeBytes: 1024, CollectedAt: now, StatsReset: &reset,
			},
			{
				Database: "app", Schema: "public", TableName: "orders", IndexName: "idx_b",
				Definition: "CREATE INDEX idx_b ON public.orders USING btree (customer_id)",
				IdxScan: 0, SizeBytes: 1024, CollectedAt: now, StatsReset: &reset,
			},
		},
	}

	stub := &serviceStub{facts: facts}
	produced, saved, err := NewService(stub, stub, "test-1.0").AnalyzeRun(context.Background(), "env-1", "run-1")
	if err != nil {
		t.Fatalf("AnalyzeRun: %v", err)
	}
	if produced == 0 || saved == 0 {
		t.Fatalf("expected findings from fixtures, produced=%d saved=%d", produced, saved)
	}
	if produced != saved {
		t.Fatalf("produced=%d saved=%d", produced, saved)
	}
	if !stub.started || stub.finished != "success" {
		t.Fatalf("lifecycle started=%v finished=%s", stub.started, stub.finished)
	}
}

// TestIndexAnalyzerUnusedRequiresObservationWindow covers T-271/T-274 window regression.
func TestIndexAnalyzerUnusedRequiresObservationWindow(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	recent := now.Add(-24 * time.Hour)

	facts := SnapshotFacts{
		EnvironmentID: "env", AuditRunID: "run",
		Indexes: []IndexFact{{
			Database: "db", Schema: "public", TableName: "t", IndexName: "idx_new",
			Definition: "CREATE INDEX idx_new ON public.t USING btree (id)",
			IdxScan: 0, CollectedAt: now, StatsReset: &recent,
		}},
	}
	out, err := IndexAnalyzer{}.Analyze(context.Background(), facts)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range out {
		if f.FindingType == "index.unused" {
			t.Fatalf("must not flag unused when observation window is too short: %+v", f)
		}
	}
}
