// Package scheduler schedules audit profiles without overlapping runs per environment.
package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mayconmendes-qc/db-auditor/internal/audit"
)

// ProfileInterval maps profile name to recurrence.
var ProfileInterval = map[string]time.Duration{
	audit.ProfileFast:    15 * time.Minute,
	audit.ProfileDaily:   24 * time.Hour,
	audit.ProfileWeekly:  7 * 24 * time.Hour,
	audit.ProfileMonthly: 30 * 24 * time.Hour,
}

// Runner is the subset of AuditRunner used by the scheduler.
type Runner interface {
	Run(ctx context.Context, environmentID, profile string) (audit.RunResult, error)
}

// Entry tracks schedule state for one environment+profile pair.
type Entry struct {
	EnvironmentID string    `json:"environment_id"`
	Profile       string    `json:"profile"`
	Enabled       bool      `json:"enabled"`
	LastRunAt     time.Time `json:"last_run_at,omitempty"`
	LastStatus    string    `json:"last_status,omitempty"`
	NextRunAt     time.Time `json:"next_run_at,omitempty"`
}

// Scheduler prevents overlap and tracks next/last execution.
type Scheduler struct {
	mu       sync.Mutex
	runner   Runner
	entries  map[string]*Entry
	inFlight map[string]struct{}
	now      func() time.Time
}

// New creates a scheduler bound to a runner.
func New(runner Runner) *Scheduler {
	return &Scheduler{
		runner:   runner,
		entries:  make(map[string]*Entry),
		inFlight: make(map[string]struct{}),
		now:      time.Now,
	}
}

func key(environmentID, profile string) string {
	return environmentID + "|" + profile
}

// UpsertSchedule enables a profile for an environment and sets next run.
func (s *Scheduler) UpsertSchedule(environmentID, profile string, enabled bool) (*Entry, error) {
	if environmentID == "" {
		return nil, fmt.Errorf("environment id required")
	}
	if _, ok := ProfileInterval[profile]; !ok && profile != audit.ProfileManual {
		return nil, fmt.Errorf("unknown profile %q", profile)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	k := key(environmentID, profile)
	e, ok := s.entries[k]
	if !ok {
		e = &Entry{EnvironmentID: environmentID, Profile: profile}
		s.entries[k] = e
	}
	e.Enabled = enabled
	if enabled && e.NextRunAt.IsZero() {
		if d, ok := ProfileInterval[profile]; ok {
			e.NextRunAt = s.now().UTC().Add(d)
		}
	}
	copy := *e
	return &copy, nil
}

// ListEntries returns a snapshot of schedule entries.
func (s *Scheduler) ListEntries() []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Entry, 0, len(s.entries))
	for _, e := range s.entries {
		out = append(out, *e)
	}
	return out
}

// TryRun starts a run if no overlap exists for environment+profile.
// Manual runs always attempt unless already in flight.
func (s *Scheduler) TryRun(ctx context.Context, environmentID, profile string) (audit.RunResult, error) {
	if environmentID == "" {
		return audit.RunResult{}, fmt.Errorf("environment id required")
	}
	if profile == "" {
		profile = audit.ProfileManual
	}
	k := key(environmentID, profile)
	s.mu.Lock()
	if _, busy := s.inFlight[k]; busy {
		s.mu.Unlock()
		return audit.RunResult{}, fmt.Errorf("run already in progress for %s/%s", environmentID, profile)
	}
	s.inFlight[k] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.inFlight, k)
		s.mu.Unlock()
	}()

	res, err := s.runner.Run(ctx, environmentID, profile)
	s.mu.Lock()
	e, ok := s.entries[k]
	if !ok {
		e = &Entry{EnvironmentID: environmentID, Profile: profile, Enabled: profile != audit.ProfileManual}
		s.entries[k] = e
	}
	e.LastRunAt = s.now().UTC()
	if err == nil {
		e.LastStatus = res.Status
	} else {
		e.LastStatus = audit.RunStatusFailed
	}
	if d, ok := ProfileInterval[profile]; ok {
		e.NextRunAt = e.LastRunAt.Add(d)
	}
	s.mu.Unlock()
	return res, err
}

// TickDue runs enabled entries whose NextRunAt is due. Returns number of runs started.
func (s *Scheduler) TickDue(ctx context.Context) int {
	now := s.now().UTC()
	s.mu.Lock()
	due := make([]Entry, 0)
	for _, e := range s.entries {
		if e.Enabled && !e.NextRunAt.IsZero() && !e.NextRunAt.After(now) {
			due = append(due, *e)
		}
	}
	s.mu.Unlock()
	started := 0
	for _, e := range due {
		if _, err := s.TryRun(ctx, e.EnvironmentID, e.Profile); err == nil {
			started++
		}
	}
	return started
}
