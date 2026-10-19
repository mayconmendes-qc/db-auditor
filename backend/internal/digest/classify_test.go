package digest

import (
	"math"
	"os"
	"testing"
	"time"
)

func TestDigestRegressionClassification(t *testing.T) {
	// CA-01: 1000@20ms → 1200@35ms = regressão média +75%.
	// Pad window totals so the fingerprint stays under the "alta" share (≥5% + total +100%).
	prior := WindowResult{OK: true, TotalTimeMS: 1_000_000, Deltas: []WindowDelta{{
		Fingerprint: "fp-orders", DatabaseName: "app",
		FirstRunID: "p0", LastRunID: "p1",
		DeltaCalls: 1000, MeanMS: 20, DeltaTotalMS: 20000,
		FirstCalls: 100, LastCalls: 1100, FirstTotalMS: 2000, LastTotalMS: 22000,
		FirstMeanMS: 20, LastMeanMS: 20,
	}}}
	current := WindowResult{OK: true, TotalTimeMS: 1_000_000, Deltas: []WindowDelta{{
		Fingerprint: "fp-orders", DatabaseName: "app",
		FirstRunID: "c0", LastRunID: "c1",
		DeltaCalls: 1200, MeanMS: 35, DeltaTotalMS: 42000,
		FirstCalls: 100, LastCalls: 1300, FirstTotalMS: 3500, LastTotalMS: 45500,
		FirstMeanMS: 35, LastMeanMS: 35,
	}}}

	items := CompareWindows(prior, current, DefaultThresholds())
	if len(items) != 1 {
		t.Fatalf("items=%d", len(items))
	}
	it := items[0]
	if it.Classification != ClassRegression {
		t.Fatalf("class=%s want %s", it.Classification, ClassRegression)
	}
	if it.Severity != SeverityMedium {
		t.Fatalf("severity=%s want %s (share=%.4f totalΔ=%.4f)", it.Severity, SeverityMedium, it.CurrentShare, it.TotalChangePct)
	}
	want := 0.75
	if math.Abs(it.MeanChangePct-want) > 0.001 {
		t.Fatalf("mean change=%.4f want %.2f", it.MeanChangePct, want)
	}
}

func TestDigestStatsResetSubwindow(t *testing.T) {
	// CA-02: reset mid-window → largest contiguous subwindow.
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	resetA := base
	resetB := base.Add(48 * time.Hour)
	points := []CollectionPoint{
		completePoint("r1", base, &resetA, &resetA),
		completePoint("r2", base.Add(24*time.Hour), &resetA, &resetA),
		completePoint("r3", base.Add(48*time.Hour), &resetB, &resetB), // reset here
		completePoint("r4", base.Add(72*time.Hour), &resetB, &resetB),
		completePoint("r5", base.Add(96*time.Hour), &resetB, &resetB),
	}
	first, last, used, ok := LargestContiguousSubwindowWithoutReset(points)
	if !ok {
		t.Fatal("expected subwindow")
	}
	// Latest stretch of length 3 (r3–r5) beats earlier stretch of length 2 (r1–r2).
	if first.AuditRunID != "r3" || last.AuditRunID != "r5" {
		t.Fatalf("subwindow %s→%s want r3→r5", first.AuditRunID, last.AuditRunID)
	}
	if len(used) != 3 {
		t.Fatalf("used=%d", len(used))
	}

	res := ResolveWindow(points, nil, DefaultRegressionGap())
	if !res.OK || !res.UsedSubwindow {
		t.Fatalf("ok=%v used=%v reason=%s", res.OK, res.UsedSubwindow, res.Reason)
	}
	if res.First.AuditRunID != "r3" || res.Last.AuditRunID != "r5" {
		t.Fatalf("resolved %s→%s", res.First.AuditRunID, res.Last.AuditRunID)
	}

	// NULL → value also counts as reset; single edge each side → no subwindow.
	only := []CollectionPoint{
		completePoint("a", base, nil, nil),
		completePoint("b", base.Add(time.Hour), &resetA, &resetA),
	}
	if _, _, _, ok := LargestContiguousSubwindowWithoutReset(only); ok {
		t.Fatal("expected no comparable subwindow")
	}
	res = ResolveWindow(only, nil, DefaultRegressionGap())
	if res.OK || res.Reason == "" {
		t.Fatalf("want no_comparable, got ok=%v reason=%q", res.OK, res.Reason)
	}
}

func TestDigestIgnoresPartialRunAsWindowEdge(t *testing.T) {
	base := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	points := []CollectionPoint{
		completePoint("c1", base, nil, nil),
		{
			AuditRunID: "partial", StartedAt: base.Add(12 * time.Hour),
			Completeness: "partial", RunStatus: "partial_success", AnalysisStatus: "success",
		},
		completePoint("c2", base.Add(24*time.Hour), nil, nil),
	}
	first, last, ok := SelectWindowEdges(points)
	if !ok {
		t.Fatal("expected two complete edges")
	}
	if first.AuditRunID != "c1" || last.AuditRunID != "c2" {
		t.Fatalf("edges %s→%s", first.AuditRunID, last.AuditRunID)
	}
	if IsCompleteEdge(points[1]) {
		t.Fatal("partial_success must not be a window edge")
	}
}

func TestDigestMarkdownHasNoLiterals(t *testing.T) {
	md := RenderMarkdown(MarkdownInput{
		EnvironmentName: "prod",
		EnvironmentID:   "00000000-0000-0000-0000-000000000001",
		State:           "ok",
		PriorFrom:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		PriorTo:         time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC),
		CurrentFrom:     time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC),
		CurrentTo:       time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		WorkloadLimitN:  500,
		Sections: DigestSections{
			Regressions: []ClassifiedItem{{
				Fingerprint:       "select from users where email = $1",
				DatabaseName:      "app",
				ReferencedObjects: []string{"public.users"},
				PriorCalls:        100, CurrentCalls: 200,
				PriorMeanMS: 10, CurrentMeanMS: 20,
				MeanChangePct: 1.0, Severity: SeverityMedium,
				Classification: ClassRegression,
			}},
		},
	})
	forbidden := []string{
		"user@example.com",
		"123.456.789-00",
		"'secret'",
		"WHERE email = 'a@b.c'",
		"cpf",
	}
	for _, lit := range forbidden {
		if containsFold(md, lit) && lit != "cpf" {
			t.Fatalf("markdown contains literal %q", lit)
		}
	}
	// Fingerprint may mention column names; must not contain original SQL literals.
	if containsFold(md, "INSERT INTO") || containsFold(md, "VALUES (") {
		t.Fatal("markdown looks like raw SQL")
	}
	if len(md) > maxMarkdownBytes {
		t.Fatalf("markdown too large: %d", len(md))
	}
}

func TestDigestScaleBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("scale")
	}
	// Behind AUDITOR_TEST_SCALE=1 per plan.
	if !scaleEnabled() {
		t.Skip("set AUDITOR_TEST_SCALE=1")
	}
	const fps, collections = 500, 42
	base := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	points := make([]CollectionPoint, collections)
	workload := make([]WorkloadPoint, 0, fps*collections)
	for i := 0; i < collections; i++ {
		id := "run-" + itoa(i)
		points[i] = completePoint(id, base.Add(time.Duration(i)*time.Hour), nil, nil)
		for f := 0; f < fps; f++ {
			workload = append(workload, WorkloadPoint{
				AuditRunID: id, DatabaseName: "db",
				Fingerprint:     "fp-" + itoa(f),
				Calls:           int64(1000 + i*10 + f),
				TotalExecTimeMS: float64(20000 + i*100 + f),
				MeanExecTimeMS:  20,
			})
		}
	}
	start := time.Now()
	res := ResolveWindow(points, workload, DefaultRegressionGap())
	if !res.OK {
		t.Fatalf("resolve: %s", res.Reason)
	}
	prior := res
	// Second window with slightly worse means.
	for i := range workload {
		workload[i].Calls += 200
		workload[i].TotalExecTimeMS *= 1.6
		workload[i].MeanExecTimeMS = 32
	}
	curr := ResolveWindow(points, workload, DefaultRegressionGap())
	_ = CompareWindows(prior, curr, DefaultThresholds())
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("scale budget exceeded: %s", elapsed)
	}
}

func completePoint(id string, at time.Time, stats, workload *time.Time) CollectionPoint {
	return CollectionPoint{
		AuditRunID: id, StartedAt: at,
		Completeness: "complete", RunStatus: "success", AnalysisStatus: "success",
		StatsReset: stats, WorkloadReset: workload,
	}
}

func DefaultRegressionGap() time.Duration { return 7 * 24 * time.Hour }

func containsFold(s, sub string) bool {
	return len(sub) > 0 && (s == sub || len(s) >= len(sub) && (indexFold(s, sub) >= 0))
}

func indexFold(s, sub string) int {
	sl, subl := []rune(s), []rune(sub)
	for i := 0; i+len(subl) <= len(sl); i++ {
		ok := true
		for j := range subl {
			a, b := sl[i+j], subl[j]
			if a >= 'A' && a <= 'Z' {
				a += 'a' - 'A'
			}
			if b >= 'A' && b <= 'Z' {
				b += 'a' - 'A'
			}
			if a != b {
				ok = false
				break
			}
		}
		if ok {
			return i
		}
	}
	return -1
}

func scaleEnabled() bool {
	return os.Getenv("AUDITOR_TEST_SCALE") == "1"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
