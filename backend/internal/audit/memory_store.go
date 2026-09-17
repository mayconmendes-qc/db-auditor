package audit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryRunStore is an in-memory RunStore for unit tests and local dry-runs.
type MemoryRunStore struct {
	mu            sync.Mutex
	AuditRuns     map[string]*memAuditRun
	CollectorRuns map[string]*memCollectorRun
}

type memAuditRun struct {
	ID               string
	EnvironmentID    string
	Profile          string
	Status           string
	ServiceVersion   string
	CollectorVersion string
	Warnings         []string
	Errors           []string
	StartedAt        time.Time
	FinishedAt       *time.Time
}

type memCollectorRun struct {
	ID            string
	AuditRunID    string
	Name          string
	Version       string
	Status        string
	Rows          int64
	Warning       string
	Error         string
	StartedAt     time.Time
	FinishedAt    *time.Time
}

// NewMemoryRunStore creates an empty memory store.
func NewMemoryRunStore() *MemoryRunStore {
	return &MemoryRunStore{
		AuditRuns:     make(map[string]*memAuditRun),
		CollectorRuns: make(map[string]*memCollectorRun),
	}
}

func (m *MemoryRunStore) StartAuditRun(_ context.Context, environmentID, profile, serviceVersion, collectorVersion string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := uuid.NewString()
	m.AuditRuns[id] = &memAuditRun{
		ID:               id,
		EnvironmentID:    environmentID,
		Profile:          profile,
		Status:           RunStatusRunning,
		ServiceVersion:   serviceVersion,
		CollectorVersion: collectorVersion,
		StartedAt:        time.Now().UTC(),
	}
	return id, nil
}

func (m *MemoryRunStore) FinishAuditRun(_ context.Context, auditRunID, status string, warnings, errs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	run, ok := m.AuditRuns[auditRunID]
	if !ok {
		return fmt.Errorf("audit run %s not found", auditRunID)
	}
	now := time.Now().UTC()
	run.Status = status
	run.Warnings = warnings
	run.Errors = errs
	run.FinishedAt = &now
	return nil
}

func (m *MemoryRunStore) StartCollectorRun(_ context.Context, auditRunID, name, version string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.AuditRuns[auditRunID]; !ok {
		return "", fmt.Errorf("audit run %s not found", auditRunID)
	}
	id := uuid.NewString()
	m.CollectorRuns[id] = &memCollectorRun{
		ID:         id,
		AuditRunID: auditRunID,
		Name:       name,
		Version:    version,
		Status:     CollectorStatusRunning,
		StartedAt:  time.Now().UTC(),
	}
	return id, nil
}

func (m *MemoryRunStore) FinishCollectorRun(_ context.Context, collectorRunID, status string, rows int64, warning, errMsg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	run, ok := m.CollectorRuns[collectorRunID]
	if !ok {
		return fmt.Errorf("collector run %s not found", collectorRunID)
	}
	now := time.Now().UTC()
	run.Status = status
	run.Rows = rows
	run.Warning = warning
	run.Error = errMsg
	run.FinishedAt = &now
	return nil
}
