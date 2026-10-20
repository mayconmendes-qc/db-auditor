package analyzer

import (
	"context"
	"testing"
)

func TestRetentionMissingAndCompressionMissing(t *testing.T) {
	facts := SnapshotFacts{
		EnvironmentID: "env", AuditRunID: "run",
		Hypertables: []HypertableFact{{
			Database: "db", Schema: "public", Name: "metrics",
			SizeBytes: 12 * 1024 * 1024 * 1024, ChunksOlderThanDays: 400,
			UncompressedOldBytes: 8 * 1024 * 1024 * 1024, CompressionEnabled: false,
		}},
	}
	got, err := (TimescaleRetentionAnalyzer{}).Analyze(context.TODO(), facts)
	if err != nil {
		t.Fatal(err)
	}
	if !hasType(got, "timescale.retention_missing") {
		t.Fatal("expected retention_missing")
	}
	if !hasType(got, "timescale.compression_missing") {
		t.Fatal("expected compression_missing")
	}
}

func TestCompressionRatioLow(t *testing.T) {
	facts := SnapshotFacts{
		EnvironmentID: "env", AuditRunID: "run",
		Hypertables: []HypertableFact{{
			Database: "db", Schema: "public", Name: "metrics",
			CompressedChunkCount: 5, ObservedCompressionRatio: 1.5,
		}},
	}
	got, err := (TimescaleRetentionAnalyzer{}).Analyze(context.TODO(), facts)
	if err != nil {
		t.Fatal(err)
	}
	if !hasType(got, "timescale.compression_ratio_low") {
		t.Fatal("expected compression_ratio_low")
	}
}

func TestSavedScenarioCaggConflictHelper(t *testing.T) {
	f := ConflictFindingForSavedScenario("env", "run", "db", "public", "metrics", "metrics_1d", 90, 120)
	if f.FindingType != "timescale.retention_cagg_conflict" || f.Severity != SeverityHigh {
		t.Fatalf("%+v", f)
	}
	if f.AuditRunID != "run" {
		t.Fatal("must link to complete run (ADR-0001 D9)")
	}
}
