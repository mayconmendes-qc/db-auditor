package tsretention

import (
	"os"
	"testing"
	"time"
)

// RNF-03: simulation under 1s for 10 000 chunks (behind AUDITOR_TEST_SCALE=1).
func TestSimulationBudget10kChunks(t *testing.T) {
	if os.Getenv("AUDITOR_TEST_SCALE") != "1" {
		t.Skip("set AUDITOR_TEST_SCALE=1")
	}
	now := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	chunks := make([]Chunk, 10_000)
	const oneGB = int64(bytesPerGB)
	for i := 0; i < len(chunks); i++ {
		end := now.Add(-time.Duration(i) * time.Hour)
		start := end.Add(-time.Hour)
		chunks[i] = Chunk{
			Name:           itoa(i),
			RangeStart:     &start,
			RangeEnd:       &end,
			TotalSizeBytes: oneGB / 10,
		}
	}
	start := time.Now()
	res := Simulate(HypertableState{Name: "metrics", Chunks: chunks, Now: now}, []Scenario{
		{Label: "ret-90", RetentionDays: 90},
		{Label: "comp-7", CompressAfterDays: 7},
		{Label: "both", RetentionDays: 180, CompressAfterDays: 14},
	})
	elapsed := time.Since(start)
	if elapsed > time.Second {
		t.Fatalf("simulation took %s want <1s (RNF-03)", elapsed)
	}
	if len(res.Scenarios) != 3 {
		t.Fatalf("scenarios=%d", len(res.Scenarios))
	}
}
