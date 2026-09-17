package audit

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// RunStore persists audit_run and collector_run lifecycle events.
type RunStore interface {
	StartAuditRun(ctx context.Context, environmentID, profile, serviceVersion, collectorVersion string) (auditRunID string, err error)
	FinishAuditRun(ctx context.Context, auditRunID, status string, warnings, errs []string) error
	StartCollectorRun(ctx context.Context, auditRunID, name, version string) (collectorRunID string, err error)
	FinishCollectorRun(ctx context.Context, collectorRunID, status string, rows int64, warning, errMsg string) error
}

// RunnerOptions configures timeouts, retries and concurrency.
type RunnerOptions struct {
	ServiceVersion   string
	CollectorVersion string
	CollectorTimeout time.Duration
	MaxRetries       int
	RetryBackoff     time.Duration
	MaxWorkers       int
}

func (o RunnerOptions) withDefaults() RunnerOptions {
	if o.ServiceVersion == "" {
		o.ServiceVersion = "0.0.0"
	}
	if o.CollectorVersion == "" {
		o.CollectorVersion = "1.0.0"
	}
	if o.CollectorTimeout <= 0 {
		o.CollectorTimeout = 2 * time.Minute
	}
	if o.MaxRetries < 0 {
		o.MaxRetries = 0
	}
	if o.RetryBackoff <= 0 {
		o.RetryBackoff = 200 * time.Millisecond
	}
	if o.MaxWorkers <= 0 {
		o.MaxWorkers = 4
	}
	return o
}

// CollectorOutcome is the result of one collector execution.
type CollectorOutcome struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Rows    int64  `json:"rows"`
	Warning string `json:"warning,omitempty"`
	Error   string `json:"error,omitempty"`
}

// RunResult is the aggregated outcome of an audit run.
type RunResult struct {
	AuditRunID string             `json:"audit_run_id"`
	Status     string             `json:"status"`
	Collectors []CollectorOutcome `json:"collectors"`
	Warnings   []string           `json:"warnings"`
	Errors     []string           `json:"errors"`
}

// Runner executes registered collectors for an environment and profile.
type Runner struct {
	registry *Registry
	store    RunStore
	opts     RunnerOptions
}

// NewRunner builds an AuditRunner.
func NewRunner(registry *Registry, store RunStore, opts RunnerOptions) *Runner {
	return &Runner{
		registry: registry,
		store:    store,
		opts:     opts.withDefaults(),
	}
}

// Run executes collectors for profile against environmentID.
func (r *Runner) Run(ctx context.Context, environmentID, profile string) (RunResult, error) {
	if environmentID == "" {
		return RunResult{}, fmt.Errorf("environment id is required")
	}
	if profile == "" {
		profile = ProfileManual
	}
	collectors := r.registry.List(profile)
	if len(collectors) == 0 {
		return RunResult{}, fmt.Errorf("no collectors registered for profile %q", profile)
	}

	auditRunID, err := r.store.StartAuditRun(ctx, environmentID, profile, r.opts.ServiceVersion, r.opts.CollectorVersion)
	if err != nil {
		return RunResult{}, fmt.Errorf("start audit run: %w", err)
	}

	outcomes := make([]CollectorOutcome, len(collectors))
	var (
		wg        sync.WaitGroup
		sem       = make(chan struct{}, r.opts.MaxWorkers)
		cancelled bool
	)

	for i, spec := range collectors {
		if ctx.Err() != nil {
			cancelled = true
			outcomes[i] = CollectorOutcome{Name: spec.Name, Status: CollectorStatusSkipped, Warning: "run cancelled"}
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, spec CollectorSpec) {
			defer wg.Done()
			defer func() { <-sem }()
			outcomes[idx] = r.runOne(ctx, auditRunID, spec)
		}(i, spec)
	}
	wg.Wait()
	if ctx.Err() != nil {
		cancelled = true
	}

	var success, failed, skipped int
	warnings := make([]string, 0)
	errs := make([]string, 0)
	for _, o := range outcomes {
		switch o.Status {
		case CollectorStatusSuccess:
			success++
		case CollectorStatusFailed:
			failed++
			if o.Error != "" {
				errs = append(errs, fmt.Sprintf("%s: %s", o.Name, o.Error))
			}
		case CollectorStatusSkipped:
			skipped++
		}
		if o.Warning != "" {
			warnings = append(warnings, fmt.Sprintf("%s: %s", o.Name, o.Warning))
		}
	}
	status := AggregateRunStatus(success, failed, skipped, cancelled)
	if err := r.store.FinishAuditRun(ctx, auditRunID, status, warnings, errs); err != nil {
		return RunResult{AuditRunID: auditRunID, Status: status, Collectors: outcomes, Warnings: warnings, Errors: errs},
			fmt.Errorf("finish audit run: %w", err)
	}
	return RunResult{
		AuditRunID: auditRunID,
		Status:     status,
		Collectors: outcomes,
		Warnings:   warnings,
		Errors:     errs,
	}, nil
}

func (r *Runner) runOne(ctx context.Context, auditRunID string, spec CollectorSpec) CollectorOutcome {
	collectorRunID, err := r.store.StartCollectorRun(ctx, auditRunID, spec.Name, spec.Version)
	if err != nil {
		return CollectorOutcome{Name: spec.Name, Status: CollectorStatusFailed, Error: err.Error()}
	}

	var (
		rows    int64
		runErr  error
		warning string
	)
	attempts := r.opts.MaxRetries + 1
	for attempt := 0; attempt < attempts; attempt++ {
		if ctx.Err() != nil {
			runErr = ctx.Err()
			break
		}
		cctx, cancel := context.WithTimeout(ctx, r.opts.CollectorTimeout)
		rows, runErr = spec.Run(cctx)
		cancel()
		if runErr == nil {
			break
		}
		if !isRetryable(runErr) || attempt == attempts-1 {
			break
		}
		warning = fmt.Sprintf("retry %d after: %v", attempt+1, runErr)
		timer := time.NewTimer(r.opts.RetryBackoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			runErr = ctx.Err()
			attempt = attempts // force exit
		case <-timer.C:
		}
	}

	status := CollectorStatusSuccess
	errMsg := ""
	if runErr != nil {
		status = CollectorStatusFailed
		errMsg = runErr.Error()
	}
	if ferr := r.store.FinishCollectorRun(ctx, collectorRunID, status, rows, warning, errMsg); ferr != nil && errMsg == "" {
		errMsg = ferr.Error()
		status = CollectorStatusFailed
	}
	return CollectorOutcome{Name: spec.Name, Status: status, Rows: rows, Warning: warning, Error: errMsg}
}

func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var t *TransientError
	return errors.As(err, &t)
}

// TransientError marks an error as safe to retry.
type TransientError struct {
	Err error
}

func (e *TransientError) Error() string {
	if e.Err == nil {
		return "transient error"
	}
	return e.Err.Error()
}

func (e *TransientError) Unwrap() error { return e.Err }
