package analyzer

import (
	"context"
	"testing"
)

func TestStorageAnalyzerTopConsumers(t *testing.T) {
	facts := SnapshotFacts{
		EnvironmentID: "env-1",
		Tables: []TableFact{
			{Database: "db", Schema: "public", Name: "a", SizeBytes: 100},
			{Database: "db", Schema: "public", Name: "b", SizeBytes: 500},
			{Database: "db", Schema: "public", Name: "c", SizeBytes: 1 << 31},
		},
	}
	out, err := StorageAnalyzer{}.Analyze(context.Background(), facts)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) < 2 {
		t.Fatalf("expected top consumers + large table, got %d", len(out))
	}
	foundLarge := false
	for _, f := range out {
		if f.FindingType == "storage.large_table" {
			foundLarge = true
		}
		if f.DedupKey == "" {
			t.Error("dedup_key required")
		}
	}
	if !foundLarge {
		t.Error("expected storage.large_table finding")
	}
}

func TestIndexAnalyzerUnusedNoDrop(t *testing.T) {
	facts := SnapshotFacts{
		EnvironmentID: "env-1",
		Indexes: []IndexFact{
			{Database: "db", Schema: "public", TableName: "t", IndexName: "idx_unused", IdxScan: 0},
			{Database: "db", Schema: "public", TableName: "t", IndexName: "idx_used", IdxScan: 10},
			{Database: "db", Schema: "public", TableName: "t", IndexName: "pk", IdxScan: 0, IsPrimary: true},
		},
	}
	out, err := IndexAnalyzer{}.Analyze(context.Background(), facts)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 unused index finding, got %d", len(out))
	}
	if out[0].FindingType != "index.unused" {
		t.Fatalf("type = %s", out[0].FindingType)
	}
	note, _ := out[0].Evidence["note"].(string)
	if note == "" {
		t.Error("expected explicit no-DROP note in evidence")
	}
}

func TestChunkAnalyzerHighCount(t *testing.T) {
	facts := SnapshotFacts{
		EnvironmentID: "env-1",
		Hypertables: []HypertableFact{
			{Database: "db", Schema: "public", Name: "metrics", NumChunks: 600},
			{Database: "db", Schema: "public", Name: "small", NumChunks: 10},
		},
	}
	out, err := ChunkAnalyzer{}.Analyze(context.Background(), facts)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 high_count finding, got %d", len(out))
	}
}

func TestRunnerAggregates(t *testing.T) {
	r := NewRunner(DefaultRegistry())
	out, err := r.Run(context.Background(), SnapshotFacts{
		EnvironmentID: "env-1",
		Tables:        []TableFact{{Database: "db", Schema: "public", Name: "t", SizeBytes: 50}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Error("expected at least storage findings")
	}
}

func TestDedupKeyStable(t *testing.T) {
	a := DedupKey("index.unused", "db.public.idx", "title")
	b := DedupKey("index.unused", "db.public.idx", "title")
	if a != b || a == "" {
		t.Fatalf("unstable dedup key: %q vs %q", a, b)
	}
}
