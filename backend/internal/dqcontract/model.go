// Package dqcontract implements versioned data-quality contracts (plan 03).
package dqcontract

import (
	"fmt"
	"strings"
)

const (
	DefaultSampleLimit = 10_000
	HardSampleCeiling  = 100_000
	MaxChecks          = 50
	DefaultFreshnessH  = 24
	DefaultVolumePct   = 50.0
	CheckTimeoutSec    = 30
	MaxConcurrentEnv   = 4
	OneGiB             = int64(1 << 30)

	StatusProposed   = "proposed"
	StatusApproved   = "approved"
	StatusSuperseded = "superseded"

	ResultPassed      = "passed"
	ResultViolated    = "violated"
	ResultNotExecuted = "not_executed"

	ModeSchedule = "schedule"
	ModeOnDemand = "on_demand"
	ModeCLI      = "cli"

	CheckNulls       = "nulls"
	CheckUniqueness  = "uniqueness"
	CheckOrphan      = "orphan"
	CheckDomain      = "domain"
	CheckFreshness   = "freshness"
	CheckVolume      = "volume"

	SeverityInfo     = "info"
	SeverityLow      = "low"
	SeverityMedium   = "medium"
	SeverityHigh     = "high"
	SeverityCritical = "critical"

	FindingType = "quality.contract_violation"
	RuleVersion = "1"
	Category    = "integrity"
)

// Document is the canonical contract payload (JSON in quality_contract_version.document).
type Document struct {
	Database        string  `json:"database" yaml:"database"`
	Schema          string  `json:"schema" yaml:"schema"`
	Table           string  `json:"table" yaml:"table"`
	SampleLimit     int     `json:"sample_limit" yaml:"sample_limit"`
	ScheduleProfile string  `json:"schedule_profile,omitempty" yaml:"schedule_profile,omitempty"`
	Checks          []Check `json:"checks" yaml:"checks"`
}

// Check is one declarative quality check.
type Check struct {
	Type          string   `json:"type" yaml:"type"`
	Column        string   `json:"column,omitempty" yaml:"column,omitempty"`
	Columns       []string `json:"columns,omitempty" yaml:"columns,omitempty"`
	MaxNullPct    *float64 `json:"max_null_pct,omitempty" yaml:"max_null_pct,omitempty"`
	Critical      bool     `json:"critical,omitempty" yaml:"critical,omitempty"`
	RefSchema     string   `json:"ref_schema,omitempty" yaml:"ref_schema,omitempty"`
	RefTable      string   `json:"ref_table,omitempty" yaml:"ref_table,omitempty"`
	RefColumn     string   `json:"ref_column,omitempty" yaml:"ref_column,omitempty"`
	Allowed       []string `json:"allowed,omitempty" yaml:"allowed,omitempty"`
	Min           *float64 `json:"min,omitempty" yaml:"min,omitempty"`
	Max           *float64 `json:"max,omitempty" yaml:"max,omitempty"`
	MinDate       string   `json:"min_date,omitempty" yaml:"min_date,omitempty"`
	MaxDate       string   `json:"max_date,omitempty" yaml:"max_date,omitempty"`
	TolerancePct  float64  `json:"tolerance_pct,omitempty" yaml:"tolerance_pct,omitempty"`
	MaxAgeHours   *float64 `json:"max_age_hours,omitempty" yaml:"max_age_hours,omitempty"`
	MaxChangePct  *float64 `json:"max_change_pct,omitempty" yaml:"max_change_pct,omitempty"`
	Severity      string   `json:"severity,omitempty" yaml:"severity,omitempty"`
}

// CheckResult is the persisted outcome of one check (counts only).
type CheckResult struct {
	CheckKey           string   `json:"check_key"`
	CheckType          string   `json:"check_type"`
	ColumnName         string   `json:"column_name"`
	Result             string   `json:"result"`
	Reason             string   `json:"reason,omitempty"`
	Severity           string   `json:"severity"`
	ObservedCount      *int     `json:"observed_count,omitempty"`
	ObservedPct        *float64 `json:"observed_pct,omitempty"`
	SampleSize         *int     `json:"sample_size,omitempty"`
	SampleMethod       string   `json:"sample_method,omitempty"`
	FreshnessAgeHours  *float64 `json:"freshness_age_hours,omitempty"`
	SamplingNote       string   `json:"sampling_note,omitempty"`
}

// SamplingNote is shown whenever a count comes from a sample (RNF-06).
const SamplingNote = "Margem de erro amostral desconhecida: a seleção por páginas ou ordem física não é uma amostra aleatória independente. As contagens descrevem somente as linhas lidas."

// ValidateDocument checks structural limits of a contract document.
func ValidateDocument(doc Document, processMaxSample int) error {
	if strings.TrimSpace(doc.Database) == "" || strings.TrimSpace(doc.Schema) == "" || strings.TrimSpace(doc.Table) == "" {
		return fmt.Errorf("database, schema e table são obrigatórios")
	}
	if len(doc.Database) > 63 || len(doc.Schema) > 63 || len(doc.Table) > 63 {
		return fmt.Errorf("identificador de tabela inválido")
	}
	if processMaxSample <= 0 {
		processMaxSample = DefaultSampleLimit
	}
	if processMaxSample > HardSampleCeiling {
		processMaxSample = HardSampleCeiling
	}
	limit := doc.SampleLimit
	if limit == 0 {
		limit = DefaultSampleLimit
	}
	if limit < 1 || limit > processMaxSample {
		return fmt.Errorf("sample_limit deve estar entre 1 e %d (teto do processo)", processMaxSample)
	}
	switch doc.ScheduleProfile {
	case "", "fast", "daily", "weekly", "monthly":
	default:
		return fmt.Errorf("schedule_profile inválido")
	}
	if len(doc.Checks) == 0 {
		return fmt.Errorf("informe ao menos uma checagem")
	}
	if len(doc.Checks) > MaxChecks {
		return fmt.Errorf("no máximo %d checagens por contrato", MaxChecks)
	}
	for i, c := range doc.Checks {
		if err := validateCheck(c); err != nil {
			return fmt.Errorf("checks[%d]: %w", i, err)
		}
	}
	return nil
}

func validateCheck(c Check) error {
	switch c.Type {
	case CheckNulls:
		if c.Column == "" {
			return fmt.Errorf("nulls exige column")
		}
		if c.MaxNullPct == nil {
			return fmt.Errorf("nulls exige max_null_pct")
		}
		if *c.MaxNullPct < 0 || *c.MaxNullPct > 100 {
			return fmt.Errorf("max_null_pct fora de 0–100")
		}
	case CheckUniqueness:
		if c.Column == "" && len(c.Columns) == 0 {
			return fmt.Errorf("uniqueness exige column ou columns")
		}
	case CheckOrphan:
		if c.Column == "" || c.RefSchema == "" || c.RefTable == "" || c.RefColumn == "" {
			return fmt.Errorf("orphan exige column, ref_schema, ref_table e ref_column")
		}
	case CheckDomain:
		if c.Column == "" {
			return fmt.Errorf("domain exige column")
		}
		hasList := len(c.Allowed) > 0
		hasNum := c.Min != nil || c.Max != nil
		hasDate := c.MinDate != "" || c.MaxDate != ""
		n := 0
		if hasList {
			n++
		}
		if hasNum {
			n++
		}
		if hasDate {
			n++
		}
		if n != 1 {
			return fmt.Errorf("domain exige exatamente um de: allowed, min/max, min_date/max_date")
		}
		if c.TolerancePct < 0 || c.TolerancePct > 100 {
			return fmt.Errorf("tolerance_pct fora de 0–100")
		}
	case CheckFreshness:
		if c.Column == "" {
			return fmt.Errorf("freshness exige column")
		}
		if c.MaxAgeHours == nil {
			return fmt.Errorf("freshness exige max_age_hours")
		}
		if *c.MaxAgeHours <= 0 {
			return fmt.Errorf("max_age_hours deve ser positivo")
		}
	case CheckVolume:
		if c.MaxChangePct == nil {
			return fmt.Errorf("volume exige max_change_pct")
		}
		if *c.MaxChangePct < 0 {
			return fmt.Errorf("max_change_pct inválido")
		}
	default:
		return fmt.Errorf("tipo de checagem desconhecido %q", c.Type)
	}
	return nil
}

// CheckKey builds a stable identity for dedup and trend.
func CheckKey(c Check) string {
	switch c.Type {
	case CheckNulls, CheckFreshness, CheckDomain:
		return c.Type + ":" + c.Column
	case CheckUniqueness:
		cols := c.Columns
		if c.Column != "" && len(cols) == 0 {
			cols = []string{c.Column}
		}
		return c.Type + ":" + strings.Join(cols, ",")
	case CheckOrphan:
		return fmt.Sprintf("%s:%s->%s.%s.%s", c.Type, c.Column, c.RefSchema, c.RefTable, c.RefColumn)
	case CheckVolume:
		return CheckVolume
	default:
		return c.Type
	}
}

// DefaultSeverity returns the severity from the rules table when unset.
func DefaultSeverity(c Check) string {
	if c.Severity != "" {
		return c.Severity
	}
	switch c.Type {
	case CheckNulls:
		if c.Critical {
			return SeverityHigh
		}
		return SeverityMedium
	case CheckUniqueness, CheckFreshness:
		return SeverityHigh
	case CheckOrphan, CheckDomain:
		return SeverityMedium
	case CheckVolume:
		return SeverityLow
	default:
		return SeverityMedium
	}
}

// NormalizeSampleLimit fills the default when omitted.
func NormalizeSampleLimit(doc *Document) {
	if doc.SampleLimit == 0 {
		doc.SampleLimit = DefaultSampleLimit
	}
}

// RedactDomainForViewer clears allowed lists (RNF-02).
func RedactDomainForViewer(doc *Document) {
	if doc == nil {
		return
	}
	for i := range doc.Checks {
		doc.Checks[i].Allowed = nil
	}
}

// SeverityRank maps severity to an ordinal for CI gates.
func SeverityRank(s string) int {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case SeverityInfo, "informativa", "informative":
		return 0
	case SeverityLow, "baixa":
		return 1
	case SeverityMedium, "média", "media":
		return 2
	case SeverityHigh, "alta":
		return 3
	case SeverityCritical, "crítica", "critica":
		return 4
	default:
		return 2
	}
}
