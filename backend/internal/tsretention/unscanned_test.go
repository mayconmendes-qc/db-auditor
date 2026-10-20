package tsretention

import (
	"testing"
	"time"
)

func i64(v int64) *int64 { return &v }

func TestUnscannedChunksApproximateAndResetAware(t *testing.T) {
	prev := []ChunkScanObservation{
		{Name: "a", SeqScan: i64(10), IdxScan: i64(5), NTupIns: i64(1)},
		{Name: "b", SeqScan: i64(3), IdxScan: i64(0), NTupIns: i64(0)},
		{Name: "c", SeqScan: i64(1), IdxScan: i64(1)},
	}
	cur := []ChunkScanObservation{
		{Name: "a", SeqScan: i64(10), IdxScan: i64(5), NTupIns: i64(1)}, // no scans
		{Name: "b", SeqScan: i64(4), IdxScan: i64(0), NTupIns: i64(0)},  // scanned
		{Name: "c", SeqScan: i64(1), IdxScan: i64(1)},                   // no scans
	}
	res := FindUnscannedChunks(prev, cur)
	if !res.Approximate || res.Label != ApproximateUnscannedLabel {
		t.Fatalf("must be approximate: %+v", res)
	}
	if len(res.Unscanned) != 2 {
		t.Fatalf("unscanned=%v want a,c", res.Unscanned)
	}
}

func TestUnscannedStatsResetNotComparable(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(24 * time.Hour)
	prev := []ChunkScanObservation{{Name: "a", SeqScan: i64(10), IdxScan: i64(0), StatsReset: &t0}}
	cur := []ChunkScanObservation{{Name: "a", SeqScan: i64(0), IdxScan: i64(0), StatsReset: &t1}}
	res := FindUnscannedChunks(prev, cur)
	if res.Comparable || res.Reason != "stats_reset" {
		t.Fatalf("reset must invalidate: %+v", res)
	}
	if len(res.Unscanned) != 0 {
		t.Fatalf("must not list unscanned after reset: %v", res.Unscanned)
	}
}

func TestUnscannedNotCollected(t *testing.T) {
	prev := []ChunkScanObservation{{Name: "legacy"}}
	cur := []ChunkScanObservation{{Name: "legacy"}}
	res := FindUnscannedChunks(prev, cur)
	if len(res.NotCollected) != 1 || len(res.Unscanned) != 0 {
		t.Fatalf("legacy counters → not collected: %+v", res)
	}
}
