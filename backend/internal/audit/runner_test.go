package audit

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestAggregateRunStatus(t *testing.T) {
	t.Parallel()
	if got := AggregateRunStatus(2, 0, 0, false); got != RunStatusSuccess {
		t.Fatalf("got %s", got)
	}
	if got := AggregateRunStatus(1, 1, 0, false); got != RunStatusPartialSuccess {
		t.Fatalf("got %s", got)
	}
	if got := AggregateRunStatus(0, 2, 0, false); got != RunStatusFailed {
		t.Fatalf("got %s", got)
	}
	if got := AggregateRunStatus(1, 0, 0, true); got != RunStatusCancelled {
		t.Fatalf("got %s", got)
	}
}

func TestRunnerSuccessAndPartial(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	_ = reg.Register(CollectorSpec{
		Name: "ok", Version: "1", Profiles: DefaultProfiles(),
		Run: func(context.Context) (int64, error) { return 3, nil },
	})
	_ = reg.Register(CollectorSpec{
		Name: "fail", Version: "1", Profiles: DefaultProfiles(),
		Run: func(context.Context) (int64, error) { return 0, errors.New("boom") },
	})
	store := NewMemoryRunStore()
	runner := NewRunner(reg, store, RunnerOptions{MaxWorkers: 2, MaxRetries: 0})
	res, err := runner.Run(context.Background(), "env-1", ProfileManual)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != RunStatusPartialSuccess {
		t.Fatalf("status=%s", res.Status)
	}
	if len(res.Collectors) != 2 {
		t.Fatalf("collectors=%d", len(res.Collectors))
	}
}

func TestRunnerRetryTransient(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	reg := NewRegistry()
	_ = reg.Register(CollectorSpec{
		Name: "flaky", Version: "1", Profiles: DefaultProfiles(),
		Run: func(context.Context) (int64, error) {
			if calls.Add(1) == 1 {
				return 0, &TransientError{Err: errors.New("temp")}
			}
			return 1, nil
		},
	})
	store := NewMemoryRunStore()
	runner := NewRunner(reg, store, RunnerOptions{MaxRetries: 2, RetryBackoff: time.Millisecond})
	res, err := runner.Run(context.Background(), "env-1", ProfileManual)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != RunStatusSuccess {
		t.Fatalf("status=%s errors=%v", res.Status, res.Errors)
	}
	if calls.Load() < 2 {
		t.Fatalf("expected retry, calls=%d", calls.Load())
	}
}

func TestRunnerTimeout(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	_ = reg.Register(CollectorSpec{
		Name: "slow", Version: "1", Profiles: DefaultProfiles(),
		Run: func(ctx context.Context) (int64, error) {
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-time.After(time.Second):
				return 1, nil
			}
		},
	})
	store := NewMemoryRunStore()
	runner := NewRunner(reg, store, RunnerOptions{CollectorTimeout: 20 * time.Millisecond, MaxRetries: 0})
	res, err := runner.Run(context.Background(), "env-1", ProfileManual)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != RunStatusFailed {
		t.Fatalf("status=%s", res.Status)
	}
}

func TestRegistryProfileFilter(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	_ = reg.Register(CollectorSpec{
		Name: "daily-only", Version: "1",
		Profiles: map[string]struct{}{ProfileDaily: {}},
		Run:      func(context.Context) (int64, error) { return 0, nil },
	})
	if n := len(reg.List(ProfileFast)); n != 0 {
		t.Fatalf("fast list=%d", n)
	}
	if n := len(reg.List(ProfileDaily)); n != 1 {
		t.Fatalf("daily list=%d", n)
	}
}
