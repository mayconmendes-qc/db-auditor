package tsretention

import (
	"math"
	"testing"
	"time"
)

func TestSimulateRetentionCA01(t *testing.T) {
	// 400 daily chunks of 1 GB; retention 90 days → ~310 GB saved.
	now := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	chunks := make([]Chunk, 400)
	const oneGB = int64(bytesPerGB)
	for i := 0; i < 400; i++ {
		end := now.Add(-time.Duration(i) * 24 * time.Hour)
		start := end.Add(-24 * time.Hour)
		chunks[i] = Chunk{
			Name:           "c" + itoa(i),
			RangeStart:     &start,
			RangeEnd:       &end,
			TotalSizeBytes: oneGB,
		}
	}
	res := Simulate(HypertableState{Name: "metrics", Chunks: chunks, Now: now}, []Scenario{{
		Label: "ret-90", RetentionDays: 90,
	}})
	if len(res.Scenarios) != 1 {
		t.Fatalf("scenarios=%d", len(res.Scenarios))
	}
	saved := res.Scenarios[0].GBSaved
	if math.Abs(saved-310) > 2 {
		t.Fatalf("GB saved=%.2f want ~310", saved)
	}
	if len(res.AgeBuckets) == 0 {
		t.Fatal("expected age buckets")
	}
}

func TestSimulateCompressionObservedRatioCA02(t *testing.T) {
	now := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	old := now.Add(-14 * 24 * time.Hour)
	chunks := []Chunk{{
		Name: "compressed", RangeEnd: &old, IsCompressed: true,
		TotalSizeBytes:         2 * bytesPerGB,
		BeforeCompressionBytes: 10 * bytesPerGB,
		AfterCompressionBytes:  2 * bytesPerGB,
	}, {
		Name: "open", RangeEnd: &old, IsCompressed: false,
		TotalSizeBytes: 10 * bytesPerGB,
	}}
	ratio, ok := ObservedCompressionRatio(chunks)
	if !ok || math.Abs(ratio-5) > 0.01 {
		t.Fatalf("ratio=%v ok=%v want 5 observed", ratio, ok)
	}
	res := Simulate(HypertableState{Chunks: chunks, Now: now}, []Scenario{{
		Label: "comp-7", CompressAfterDays: 7,
	}})
	if res.RatioConfidenceLow {
		t.Fatal("confidence should not be low when ratio observed")
	}
	if math.Abs(res.ObservedRatio-5) > 0.01 {
		t.Fatalf("observed=%v", res.ObservedRatio)
	}
}

func TestSimulateDefaultRatioLowConfidenceCA03(t *testing.T) {
	now := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	old := now.Add(-14 * 24 * time.Hour)
	chunks := []Chunk{{
		Name: "raw", RangeEnd: &old, IsCompressed: false, TotalSizeBytes: bytesPerGB,
	}}
	res := Simulate(HypertableState{Chunks: chunks, Now: now}, []Scenario{{
		Label: "comp-7", CompressAfterDays: 7,
	}})
	if !res.RatioConfidenceLow {
		t.Fatal("expected low confidence")
	}
	if math.Abs(res.ObservedRatio-DefaultCompressionRatio) > 0.01 {
		t.Fatalf("ratio=%v", res.ObservedRatio)
	}
}

func TestIntegerDimensionRetentionNotApplicable(t *testing.T) {
	now := time.Now().UTC()
	end := int64(100)
	res := Simulate(HypertableState{
		IntegerDimensionOnly: true,
		Chunks:               []Chunk{{Name: "i", RangeEndInteger: &end, TotalSizeBytes: bytesPerGB}},
		Now:                  now,
	}, []Scenario{{Label: "ret", RetentionDays: 30}})
	if res.RetentionApplicable {
		t.Fatal("retention must be N/A for integer dimension")
	}
}

func TestInventoryTruncatedMarksPartial(t *testing.T) {
	res := Simulate(HypertableState{InventoryTruncated: true}, nil)
	if !res.Partial {
		t.Fatal("expected partial")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
