package restoreverify

import (
	"encoding/json"
	"time"
)

const KindTargetRestoreDrill = "target_restore_drill"

const (
	StatusOK       = "ok"
	StatusFailed   = "failed"
	StatusDisabled = "disabled"
	StatusNotRun   = "not_executed"
	StatusSkipped  = "skipped"
	StatusDiverged = "diverged"
)

type Options struct {
	BackupPath           string
	ExpectedSHA256       string
	EnvironmentID        string
	Runtime              string
	Image                string
	ConfirmIsolated      bool
	Submit               bool
	SampleHash           bool
	SourceDSN            string
	APIURL               string
	CLIToken             string
	CLIVersion           string
	Timeout              time.Duration
	TargetFingerprints   []string
	SourceMajor          int
	Inventory            []InventoryObject
	EstimatedRows        []TableEstimate
	WriteCounters        []TableWriteCounter
	DiskBytesRequired    int64
	SkipContainerCleanup bool
	RuntimeExec          Runtime
}

type InventoryObject struct {
	ObjectType string `json:"object_type"`
	Key        string `json:"key"`
	Name       string `json:"name"`
}

type TableEstimate struct {
	Schema      string `json:"schema"`
	Table       string `json:"table"`
	RowEstimate int64  `json:"row_estimate"`
	Comparable  bool   `json:"comparable"`
}

type TableWriteCounter struct {
	Schema  string `json:"schema"`
	Table   string `json:"table"`
	NTupIns int64  `json:"n_tup_ins"`
	NTupUpd int64  `json:"n_tup_upd"`
	NTupDel int64  `json:"n_tup_del"`
}

type Divergence struct {
	Table      string `json:"table"`
	Kind       string `json:"kind"`
	Severity   string `json:"severity"`
	Expected   *int64 `json:"expected,omitempty"`
	Actual     *int64 `json:"actual,omitempty"`
	Detail     string `json:"detail,omitempty"`
	ObjectType string `json:"object_type,omitempty"`
}

type CheckResult struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Duration int    `json:"duration_seconds"`
	Detail   string `json:"detail,omitempty"`
}

type Evidence struct {
	Kind               string          `json:"kind"`
	EnvironmentID      string          `json:"environment_id"`
	ArtifactSHA256     string          `json:"artifact_sha256"`
	Succeeded          bool            `json:"succeeded"`
	RTOSeconds         *int            `json:"rto_seconds"`
	PrevalidateSeconds *int            `json:"prevalidate_seconds,omitempty"`
	VerifySeconds      *int            `json:"verify_seconds,omitempty"`
	Checks             []CheckResult   `json:"checks"`
	Divergences        []Divergence    `json:"divergences"`
	CLIVersion         string          `json:"cli_version"`
	Note               string          `json:"note,omitempty"`
	PerformedAt        time.Time       `json:"performed_at"`
	Integrity          json.RawMessage `json:"integrity,omitempty"`
}

type Result struct {
	ExitCode int
	Evidence Evidence
	Summary  string
	Err      error
}

const (
	CountTolerancePct = 0.005
	CountToleranceAbs = int64(1000)
	SampleHashLimit   = 1000
	SampleHashTables  = 200
	DefaultTimeout    = 2 * time.Hour
	DefaultMaxAgeDays = 30
)
