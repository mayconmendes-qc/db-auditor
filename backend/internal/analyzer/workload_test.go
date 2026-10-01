package analyzer

import (
	"context"
	"testing"
	"time"
)

func TestPerformanceAnalyzerCorrelatesReliableWorkloadToTable(t *testing.T) {
	reset := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	out, err := (PerformanceAnalyzer{}).Analyze(context.Background(), SnapshotFacts{
		EnvironmentID: "env", AuditRunID: "run",
		QueryStats: []QueryStatFact{{Database: "db", QueryFingerprint: "sha256:abc", Calls: 20,
			MeanExecTimeMs: 800, TotalExecTimeMs: 16000, ReferencedObjects: []string{"public.orders"},
			EvidenceQuality: "object_reference", StatsReset: &reset, PgStatStatements: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].ObjectType != "table" || out[0].ObjectName != "public.orders" {
		t.Fatalf("expected table-correlated finding, got %#v", out)
	}
	if out[0].Evidence["observation_window_start"] == "unknown" {
		t.Fatal("expected observation window in evidence")
	}
}
