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
	Database    string    `json:"database"`
	Schema      string    `json:"schema"`
	Name        string    `json:"name"`
	SizeBytes   int64     `json:"size_bytes"`
	CollectedAt time.Time `json:"collected_at,omitempty"`
}

// IndexFact carries scan counters when available.
type IndexFact struct {
	Database   string `json:"database"`
	Schema     string `json:"schema"`
	TableName  string `json:"table_name"`
	IndexName  string `json:"index_name"`
	IdxScan    int64  `json:"idx_scan"`
	SizeBytes  int64  `json:"size_bytes"`
	IsPrimary  bool   `json:"is_primary"`
	IsUnique   bool   `json:"is_unique"`
	Definition string `json:"definition"`
}

// HypertableFact summarizes chunk topology.
type HypertableFact struct {
	Database  string `json:"database"`
	Schema    string `json:"schema"`
	Name      string `json:"name"`
	NumChunks int    `json:"num_chunks"`
	SizeBytes int64  `json:"size_bytes"`
}

// ChunkFact is a single chunk size observation.
type ChunkFact struct {
	Database       string `json:"database"`
	Schema         string `json:"schema"`
	HypertableName string `json:"hypertable_name"`
	ChunkName      string `json:"chunk_name"`
	SizeBytes      int64  `json:"size_bytes"`
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
