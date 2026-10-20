package migrationrisk

// RulesVersion identifies the MR catalog embedded in this binary.
const RulesVersion = "mr-1"

// SchemaVersion tags JSON/Markdown analysis payloads.
const SchemaVersion = "1"

// MaxOperations is the hard cap per analysis run (RNF-03).
const MaxOperations = 500

// Severity levels used by MR rules and the CI gate.
const (
	SeverityInfo     = "info"
	SeverityLow      = "low"
	SeverityMedium   = "medium"
	SeverityHigh     = "high"
	SeverityCritical = "critical"
)

// Confidence labels.
const (
	ConfidenceLow    = "low"
	ConfidenceMedium = "medium"
	ConfidenceHigh   = "high"
)

// Operation is one DDL statement classified by the parser.
type Operation struct {
	File            string `json:"file"`
	Line            int    `json:"line"`
	Kind            string `json:"kind"`
	Object          string `json:"object,omitempty"`
	Object2         string `json:"object2,omitempty"` // FK referenced table
	RawNorm         string `json:"-"`                 // literal-stripped, never exported
	Concurrent      bool   `json:"concurrent,omitempty"`
	NotValid        bool   `json:"not_valid,omitempty"`
	UsingIndex      bool   `json:"using_index,omitempty"`
	InTxn           bool   `json:"in_transaction,omitempty"`
	CreatedHere     bool   `json:"created_here,omitempty"`
	ParseError      string `json:"parse_error,omitempty"`
	VolatileDefault bool   `json:"volatile_default,omitempty"`
	HasLockTimeout  bool   `json:"-"`
}

// Finding is one MR-* hit (or an unanalyzed marker).
type Finding struct {
	Code          string   `json:"code"`
	File          string   `json:"file"`
	Line          int      `json:"line"`
	Operation     string   `json:"operation"`
	Object        string   `json:"object,omitempty"`
	Lock          string   `json:"lock,omitempty"`
	RewriteScan   string   `json:"rewrite_or_scan,omitempty"`
	SizeBytes     *int64   `json:"size_bytes,omitempty"`
	RowEstimate   *int64   `json:"row_estimate,omitempty"`
	WriteRate     *float64 `json:"write_rate_per_s,omitempty"`
	DurationEst   string   `json:"duration_estimate,omitempty"`
	Severity      string   `json:"severity"`
	Confidence    string   `json:"confidence"`
	Impact        string   `json:"impact"`
	Alternative   string   `json:"alternative,omitempty"`
	Suppressed    bool     `json:"suppressed"`
	Justification string   `json:"justification,omitempty"`
	Unanalyzed    bool     `json:"unanalyzed,omitempty"`
}

// Report is the schema_versioned analysis payload.
type Report struct {
	SchemaVersion string    `json:"schema_version"`
	RulesVersion  string    `json:"rules_version"`
	PGVersion     int       `json:"pg_version"`
	ContextMode   string    `json:"context_mode"` // none | snapshot | direct
	Partial       bool      `json:"partial,omitempty"`
	Truncated     bool      `json:"truncated,omitempty"`
	AuditRunID    string    `json:"audit_run_id,omitempty"`
	AuditRunAt    string    `json:"audit_run_at,omitempty"`
	Findings      []Finding `json:"findings"`
	Summary       Summary   `json:"summary"`
}

// Summary counts findings by severity (non-suppressed).
type Summary struct {
	Info     int    `json:"info"`
	Low      int    `json:"low"`
	Medium   int    `json:"medium"`
	High     int    `json:"high"`
	Critical int    `json:"critical"`
	Highest  string `json:"highest"`
}

// TableContext carries snapshot or direct-read metadata for one table.
type TableContext struct {
	Qualified    string
	SizeBytes    int64
	RowEstimate  int64
	WriteRate    float64 // writes/s when comparable; 0 unknown
	HasWriteRate bool
}

// Context holds environment-level analysis context.
type Context struct {
	Mode       string // none | snapshot | direct
	PGVersion  int
	Partial    bool
	Stale      bool
	Missing    bool
	AuditRunID string
	AuditRunAt string
	Tables     map[string]TableContext
}

// Thresholds are per-environment calibratable gates (initial values from the spec).
type Thresholds struct {
	HighSizeBytes       int64
	CriticalSizeBytes   int64
	HighRows            int64
	CriticalRows        int64
	WriteRateHigh       float64
	ThroughputBytesPerS int64
	MaxContextAgeDays   int
	FailOn              string
}

// DefaultThresholds returns the initial calibrated values from the spec.
func DefaultThresholds() Thresholds {
	return Thresholds{
		HighSizeBytes:       1 << 30,  // 1 GiB
		CriticalSizeBytes:   10 << 30, // 10 GiB
		HighRows:            5_000_000,
		CriticalRows:        50_000_000,
		WriteRateHigh:       100,
		ThroughputBytesPerS: 50 << 20, // 50 MiB/s
		MaxContextAgeDays:   7,
		FailOn:              SeverityHigh,
	}
}
