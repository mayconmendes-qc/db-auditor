// Package analyzer turns inventory snapshots into actionable findings.
package analyzer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// Severity ranks finding impact.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// Status is the triage state of a finding.
type Status string

const (
	StatusOpen         Status = "open"
	StatusAcknowledged Status = "acknowledged"
	StatusResolved     Status = "resolved"
	StatusSuppressed   Status = "suppressed"
)

// Finding is a diagnostic produced by an analyzer.
type Finding struct {
	ID            string         `json:"id,omitempty"`
	EnvironmentID string         `json:"environment_id"`
	AuditRunID    string         `json:"audit_run_id,omitempty"`
	FindingType   string         `json:"finding_type"`
	Severity      Severity       `json:"severity"`
	Status        Status         `json:"status"`
	Title         string         `json:"title"`
	Summary       string         `json:"summary"`
	ObjectType    string         `json:"object_type,omitempty"`
	ObjectKey     string         `json:"object_key,omitempty"`
	DatabaseName  string         `json:"database_name,omitempty"`
	SchemaName    string         `json:"schema_name,omitempty"`
	ObjectName    string         `json:"object_name,omitempty"`
	Evidence      map[string]any `json:"evidence,omitempty"`
	DedupKey      string         `json:"dedup_key"`
	FirstSeenAt   time.Time      `json:"first_seen_at,omitempty"`
	LastSeenAt    time.Time      `json:"last_seen_at,omitempty"`
	ResolvedAt    *time.Time     `json:"resolved_at,omitempty"`
	Notes         string         `json:"notes,omitempty"`
}

// SnapshotFacts is the read-only input analyzers consume.
type SnapshotFacts struct {
	EnvironmentID string
	AuditRunID    string
	Tables        []TableFact
	Indexes       []IndexFact
	Hypertables   []HypertableFact
	Chunks        []ChunkFact
}

// TableFact is a minimal table size fact for storage analysis.
type TableFact struct {
	Database   string
	Schema     string
	Name       string
	SizeBytes  int64
	CollectedAt time.Time
}

// IndexFact carries scan counters when available.
type IndexFact struct {
	Database   string
	Schema     string
	TableName  string
	IndexName  string
	IdxScan    int64
	SizeBytes  int64
	IsPrimary  bool
	IsUnique   bool
	Definition string
}

// HypertableFact summarizes chunk topology.
type HypertableFact struct {
	Database   string
	Schema     string
	Name       string
	NumChunks  int
	SizeBytes  int64
}

// ChunkFact is a single chunk size observation.
type ChunkFact struct {
	Database       string
	Schema         string
	HypertableName string
	ChunkName      string
	SizeBytes      int64
}

// Analyzer produces findings from snapshot facts.
type Analyzer interface {
	Name() string
	Analyze(ctx context.Context, facts SnapshotFacts) ([]Finding, error)
}

// DedupKey builds a stable identity for first_seen/last_seen tracking.
func DedupKey(findingType, objectKey, title string) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s", findingType, objectKey, title)))
	return hex.EncodeToString(h[:16])
}

// EvidenceJSON marshals evidence for persistence.
func EvidenceJSON(ev map[string]any) []byte {
	if ev == nil {
		return []byte("{}")
	}
	b, err := json.Marshal(ev)
	if err != nil {
		return []byte("{}")
	}
	return b
}
