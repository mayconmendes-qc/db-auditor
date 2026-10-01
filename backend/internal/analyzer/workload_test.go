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
	if len(out) != 1 || out[0].ObjectType != "table" || out[0].SchemaName != "public" || out[0].ObjectName != "orders" {
		t.Fatalf("expected table-correlated finding, got %#v", out)
	}
	if out[0].Evidence["observation_window_start"] == "unknown" {
		t.Fatal("expected observation window in evidence")
	}
}

func TestPerformanceAnalyzerWorkloadPatternsRequireReliableObject(t *testing.T) {
	base := QueryStatFact{Database: "db", QueryFingerprint: "sha256:abc", QueryKind: "select", Calls: 200,
		SharedBlocksRead: 5000, SharedBlocksHit: 100, TotalExecTimeMs: 100000, MeanExecTimeMs: 500,
		ReferencedObjects: []string{"public.orders"}, EvidenceQuality: "object_reference"}
	items, err := PerformanceAnalyzer{}.Analyze(context.Background(), SnapshotFacts{EnvironmentID: "env", QueryStats: []QueryStatFact{base}})
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]bool{}
	for _, item := range items {
		kinds[item.FindingType] = true
		if item.ObjectType != "table" || item.Evidence["evidence_quality"] != "object_reference" {
			t.Fatalf("unreliable correlation: %#v", item)
		}
	}
	if !kinds["performance.workload_scan"] || !kinds["performance.workload_cost"] {
		t.Fatalf("missing patterns: %#v", kinds)
	}
	base.ReferencedObjects = []string{"orders"}
	items, err = PerformanceAnalyzer{}.Analyze(context.Background(), SnapshotFacts{QueryStats: []QueryStatFact{base}})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.ObjectType == "table" {
			t.Fatalf("ambiguous table attribution: %#v", item)
		}
	}
	base.QueryKind = "update"
	base.ReferencedObjects = []string{"public.orders"}
	base.Calls = 1500
	items, err = PerformanceAnalyzer{}.Analyze(context.Background(), SnapshotFacts{QueryStats: []QueryStatFact{base}})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range items {
		if item.FindingType == "performance.workload_write" {
			found = true
		}
	}
	if !found {
		t.Fatal("write pattern missing")
	}
}
