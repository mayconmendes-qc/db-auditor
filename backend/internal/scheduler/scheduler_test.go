package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/mayconmendes-qc/db-auditor/internal/audit"
)

type stubRunner struct {
	calls int
}

func (s *stubRunner) Run(context.Context, string, string) (audit.RunResult, error) {
	s.calls++
	return audit.RunResult{AuditRunID: "r1", Status: audit.RunStatusSuccess}, nil
}

func TestOverlapRejected(t *testing.T) {
	t.Parallel()
	r := &stubRunner{}
	sch := New(r)
	sch.inFlight[key("e1", audit.ProfileDaily)] = struct{}{}
	_, err := sch.TryRun(context.Background(), "e1", audit.ProfileDaily)
	if err == nil {
		t.Fatal("expected overlap error")
	}
}

func TestManualRunAndNext(t *testing.T) {
	t.Parallel()
	r := &stubRunner{}
	sch := New(r)
	fixed := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	sch.now = func() time.Time { return fixed }
	res, err := sch.TryRun(context.Background(), "e1", audit.ProfileDaily)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != audit.RunStatusSuccess {
		t.Fatalf("status=%s", res.Status)
	}
	entries := sch.ListEntries()
	if len(entries) != 1 {
		t.Fatalf("entries=%d", len(entries))
	}
	if !entries[0].NextRunAt.Equal(fixed.Add(24 * time.Hour)) {
		t.Fatalf("next=%v", entries[0].NextRunAt)
	}
}

func TestTickDue(t *testing.T) {
	t.Parallel()
	r := &stubRunner{}
	sch := New(r)
	fixed := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	sch.now = func() time.Time { return fixed }
	_, _ = sch.UpsertSchedule("e1", audit.ProfileFast, true)
	sch.mu.Lock()
	sch.entries[key("e1", audit.ProfileFast)].NextRunAt = fixed.Add(-time.Minute)
	sch.mu.Unlock()
	n := sch.TickDue(context.Background())
	if n != 1 || r.calls != 1 {
		t.Fatalf("started=%d calls=%d", n, r.calls)
	}
}
