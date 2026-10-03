package analyzer

import (
	"context"
	"testing"
)

func TestRetentionShorterThanChunk(t *testing.T) {
	t.Parallel()
	facts := SnapshotFacts{EnvironmentID: "e", AuditRunID: "r",
		Hypertables: []HypertableFact{{Database: "db", Schema: "public", Name: "metrics", ChunkInterval: "7 days", SizeBytes: 10}},
		Policies:    []PolicyFact{{Database: "db", HypertableSchema: "public", HypertableName: "metrics", PolicyType: "retention", Config: `{"drop_after":"1 day"}`}},
	}
	got, err := PolicyAnalyzer{}.Analyze(context.Background(), facts)
	if err != nil {
		t.Fatal(err)
	}
	if !hasType(got, "policy.retention_chunk_mismatch") || hasType(got, "policy.missing_retention") {
		t.Fatalf("%#v", typesOf(got))
	}
}

func TestCompressionPolicyNotApplied(t *testing.T) {
	t.Parallel()
	facts := SnapshotFacts{EnvironmentID: "e", AuditRunID: "r",
		Hypertables: []HypertableFact{{Database: "db", Schema: "public", Name: "metrics", SizeBytes: 10, UncompressedClosedChunks: 4}},
		Policies:    []PolicyFact{{Database: "db", HypertableSchema: "public", HypertableName: "metrics", PolicyType: "compression"}},
	}
	got, _ := PolicyAnalyzer{}.Analyze(context.Background(), facts)
	if !hasType(got, "policy.compression_not_applied") || hasType(got, "policy.missing_compression") {
		t.Fatalf("%#v", typesOf(got))
	}
}

func TestJobSLOAndWorkers(t *testing.T) {
	t.Parallel()
	facts := SnapshotFacts{EnvironmentID: "e", AuditRunID: "r", Jobs: []JobFact{{
		Database: "db", JobID: 7, ProcName: "policy_compression", LastRunStatus: "success",
		ScheduleInterval: "15 minutes", LastRunDuration: "40 minutes", Scheduled: true,
		MaxBackgroundWorkers: 2, ScheduledJobCount: 5,
	}}}
	got, _ := PolicyAnalyzer{}.Analyze(context.Background(), facts)
	if !hasType(got, "job.slo_exceeded") || !hasType(got, "job.workers_saturated") {
		t.Fatalf("%#v", typesOf(got))
	}
}

func TestChunkSkewFromAggregate(t *testing.T) {
	t.Parallel()
	facts := SnapshotFacts{EnvironmentID: "e", AuditRunID: "r", ChunksTruncated: true, ChunkStats: []ChunkStat{{
		Database: "db", Schema: "public", HypertableName: "metrics", Count: 40000, MaxBytes: 500, SumBytes: 1000,
	}}}
	got, _ := ChunkAnalyzer{}.Analyze(context.Background(), facts)
	if !hasType(got, "chunk.size_skew") || !hasType(got, "chunk.inventory_truncated") {
		t.Fatalf("%#v", typesOf(got))
	}
}

func TestCAGGLag(t *testing.T) {
	t.Parallel()
	facts := SnapshotFacts{CAGGs: []CAGGFact{{Database: "db", Schema: "public", ViewName: "hourly", HasRefreshPolicy: true, Lag: "3 hours"}}}
	got, _ := CAGGAnalyzer{}.Analyze(context.Background(), facts)
	if !hasType(got, "cagg.materialization_lag") {
		t.Fatalf("%#v", typesOf(got))
	}
}

func TestAuditorNotReadonly(t *testing.T) {
	t.Parallel()
	writable, _ := SecurityAnalyzer{}.Analyze(context.Background(), SnapshotFacts{Roles: []RoleFact{{Database: "db", RoleName: "auditor", Current: true, CanWrite: true}}})
	if !hasType(writable, "security.auditor_not_readonly") {
		t.Fatal("expected writable finding")
	}
	safe, _ := SecurityAnalyzer{}.Analyze(context.Background(), SnapshotFacts{Roles: []RoleFact{{Database: "db", RoleName: "auditor", Current: true}}})
	if hasType(safe, "security.auditor_not_readonly") {
		t.Fatal("read-only role must not be flagged")
	}
}

func TestReorderNeedsWorkload(t *testing.T) {
	t.Parallel()
	quiet, _ := PolicyAnalyzer{}.Analyze(context.Background(), SnapshotFacts{Hypertables: []HypertableFact{{Database: "db", Schema: "public", Name: "metrics"}}})
	if hasType(quiet, "policy.reorder_hypothesis") {
		t.Fatal("no workload, no reorder finding")
	}
}

func hasType(items []Finding, kind string) bool {
	for _, item := range items {
		if item.FindingType == kind {
			return true
		}
	}
	return false
}

func typesOf(items []Finding) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.FindingType)
	}
	return out
}
