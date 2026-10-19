package dqcontract

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/osmendes/db-auditor/internal/observability"
)

var (
	envSemMu sync.Mutex
	envSem   = map[string]chan struct{}{}
)

func envLimiter(env string) chan struct{} {
	envSemMu.Lock()
	defer envSemMu.Unlock()
	ch, ok := envSem[env]
	if !ok {
		ch = make(chan struct{}, MaxConcurrentEnv)
		envSem[env] = ch
	}
	return ch
}

// ProcessMaxSample returns AUDITOR_DATA_QUALITY_MAX_SAMPLE (default 10000, max 100000).
func ProcessMaxSample() int {
	raw := strings.TrimSpace(os.Getenv("AUDITOR_DATA_QUALITY_MAX_SAMPLE"))
	if raw == "" {
		return DefaultSampleLimit
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return DefaultSampleLimit
	}
	if n > HardSampleCeiling {
		return HardSampleCeiling
	}
	return n
}

// DataQualityEnabled reports whether AUDITOR_DATA_QUALITY_ENABLED=1.
func DataQualityEnabled() bool {
	return os.Getenv("AUDITOR_DATA_QUALITY_ENABLED") == "1"
}

// RunRequest drives one contract execution.
type RunRequest struct {
	EnvironmentID string
	DSN           string
	Document      Document
	Mode          string
	Volume        VolumeContext
	NullFraction  map[string]float64 // optional pre-check from column_stat_snapshot
}

// RunOutcome is the in-memory result of executing a contract document.
type RunOutcome struct {
	Status   string
	Coverage string
	Results  []CheckResult
	Started  time.Time
	Finished time.Time
}

// Execute runs all checks with the per-environment concurrency cap.
func Execute(ctx context.Context, req RunRequest) (RunOutcome, error) {
	out := RunOutcome{Started: time.Now().UTC(), Status: "success", Coverage: "complete"}
	if !DataQualityEnabled() {
		out.Status = "skipped"
		out.Coverage = "none"
		out.Finished = time.Now().UTC()
		return out, fmt.Errorf("quality disabled")
	}
	NormalizeSampleLimit(&req.Document)
	if err := ValidateDocument(req.Document, ProcessMaxSample()); err != nil {
		return out, err
	}
	lim := envLimiter(req.EnvironmentID)
	select {
	case lim <- struct{}{}:
		defer func() { <-lim }()
	case <-ctx.Done():
		out.Status = "failed"
		out.Finished = time.Now().UTC()
		return out, ctx.Err()
	}

	doc := req.Document
	t, closeFn, err := OpenTarget(ctx, req.DSN, doc.Database, doc.Schema, doc.Table, doc.SampleLimit)
	if err != nil {
		out.Status = "failed"
		out.Coverage = "none"
		out.Finished = time.Now().UTC()
		return out, err
	}
	defer closeFn()

	results := make([]CheckResult, 0, len(doc.Checks))
	violated, notExec := 0, 0
	for _, check := range doc.Checks {
		start := time.Now()
		// Optional null_fraction pre-check: if stats already exceed max, still confirm on sample.
		res := RunCheck(ctx, t, check, req.Volume)
		dur := time.Since(start)
		observability.DefaultMetrics.ObserveQualityContractCheck(res.Result, dur)
		results = append(results, res)
		switch res.Result {
		case ResultViolated:
			violated++
		case ResultNotExecuted:
			notExec++
		}
	}
	out.Results = results
	out.Finished = time.Now().UTC()
	switch {
	case violated > 0 && notExec > 0:
		out.Status = "partial_success"
		out.Coverage = "partial"
	case violated > 0:
		out.Status = "success"
		out.Coverage = "complete"
	case notExec == len(results):
		out.Status = "partial_success"
		out.Coverage = "partial"
	case notExec > 0:
		out.Status = "partial_success"
		out.Coverage = "partial"
	default:
		out.Status = "success"
		out.Coverage = "complete"
	}
	return out, nil
}

// GateExit returns CLI exit code for a run given fail-on severity.
func GateExit(results []CheckResult, failOn string) int {
	threshold := SeverityRank(failOn)
	hasNotExec := false
	for _, r := range results {
		if r.Result == ResultNotExecuted {
			hasNotExec = true
			continue
		}
		if r.Result == ResultViolated && SeverityRank(r.Severity) >= threshold {
			return 1
		}
	}
	if hasNotExec {
		return 3
	}
	return 0
}
