package loganomaly

import "time"

// SchemaVersion tags JSON/Markdown and last-login payloads.
const SchemaVersion = "1"

// Limits (RNF-03).
const (
	MaxLineBytes     = 64 * 1024
	MaxTemplates     = 10_000
	DefaultMemBudget = 256 << 20 // 256 MiB advisory
	InvalidLineRatio = 0.10
)

// Profiles.
const (
	ProfilePostgresJSONLog = "postgres-jsonlog"
	ProfileGenerico        = "generico"
)

// Severity levels (gate uses these).
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

// Anomaly kinds (RF-05).
const (
	AnomalySpike       = "spike"
	AnomalyStrongSpike = "strong_spike"
	AnomalyNewTemplate = "new_template"
	AnomalyErrorSpike  = "error_spike"
	AnomalyMissing     = "missing_template"
)

// Level buckets for counting.
const (
	LevelError   = "error"
	LevelWarning = "warning"
	LevelInfo    = "info"
)

// UnclassifiableID marks templates with no fixed tokens (RNF-02).
const UnclassifiableID = "unclassifiable"

// OthersTemplateID aggregates overflow beyond MaxTemplates.
const OthersTemplateID = "others"

// Event is one parsed, masked log line ready for templating/counting.
// Raw message is never stored.
type Event struct {
	Time           time.Time
	Level          string // normalized: error|warning|info
	RawLevel       string // original severity string (ERROR, LOG, …)
	TemplateID     string
	Template       string
	RoleHint       string // postgres-jsonlog user field only (last-login); never emitted in anomaly output
	IsConnAuth     bool   // connection authorized event
	Unclassifiable bool
}

// ReadStats reports line processing (RNF-05).
type ReadStats struct {
	LinesRead      int64 `json:"lines_read"`
	LinesInvalid   int64 `json:"lines_invalid"`
	LinesOversized int64 `json:"lines_oversized"`
	LinesParsed    int64 `json:"lines_parsed"`
	FilesRead      int   `json:"files_read"`
}

// InvalidRatio returns invalid/oversized over total lines read.
func (s ReadStats) InvalidRatio() float64 {
	if s.LinesRead == 0 {
		return 0
	}
	return float64(s.LinesInvalid+s.LinesOversized) / float64(s.LinesRead)
}

// Thresholds are initial calibratable values from the spec §3.
type Thresholds struct {
	Window           time.Duration
	BaselineWindows  int // K
	MinBaselineWins  int // < this → insufficient (default 6)
	SpikeZ           float64
	SpikeMinCount    int
	StrongFactor     float64
	StrongMinCount   int
	NewMinCount      int
	ErrorSpikeFactor float64
	MissingBaseFrac  float64
	MissingAbsentN   int
	FailOn           string
}

// DefaultThresholds returns spec initial values.
func DefaultThresholds() Thresholds {
	return Thresholds{
		Window:           5 * time.Minute,
		BaselineWindows:  12,
		MinBaselineWins:  6,
		SpikeZ:           3,
		SpikeMinCount:    20,
		StrongFactor:     8,
		StrongMinCount:   50,
		NewMinCount:      10,
		ErrorSpikeFactor: 5,
		MissingBaseFrac:  0.5,
		MissingAbsentN:   3,
		FailOn:           SeverityHigh,
	}
}

// Options configures a local analysis run (no network).
type Options struct {
	Profile      string
	Paths        []string // empty → stdin
	TimeField    string   // generico
	LevelField   string
	MessageField string
	Window       time.Duration
	BaselineK    int
	BaselineFrom time.Time // zero = use trailing K windows
	BaselineTo   time.Time
	FailOn       string
	Format       string // md|json
	LastLoginOut string // path or empty; "-" for stdout section only via report
	Thresholds   Thresholds
}
