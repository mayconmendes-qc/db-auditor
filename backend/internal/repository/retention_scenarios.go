package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/osmendes/db-auditor/internal/costlens"
	"github.com/osmendes/db-auditor/internal/tsretention"
)

// RetentionScenario is a saved what-if (spec 09).
type RetentionScenario struct {
	ID                 string     `json:"id"`
	EnvironmentID      string     `json:"environment_id"`
	DatabaseName       string     `json:"database_name"`
	SchemaName         string     `json:"schema_name"`
	HypertableName     string     `json:"hypertable_name"`
	Label              string     `json:"label"`
	RetentionDays      int        `json:"retention_days"`
	CompressAfterDays  int        `json:"compress_after_days"`
	AuditRunID         *string    `json:"audit_run_id,omitempty"`
	CurrentBytes       int64      `json:"current_bytes"`
	BytesAfter         int64      `json:"bytes_after"`
	BytesSaved         int64      `json:"bytes_saved"`
	GBSaved            float64    `json:"gb_saved"`
	Currency           string     `json:"currency,omitempty"`
	MoneySaved         *float64   `json:"money_saved,omitempty"`
	PriceVersionID     *string    `json:"price_version_id,omitempty"`
	PriceEffectiveFrom *time.Time `json:"price_effective_from,omitempty"`
	Risks              []string   `json:"risks"`
	RatioConfidenceLow bool       `json:"ratio_confidence_low"`
	ObservedRatio      *float64   `json:"observed_ratio,omitempty"`
	Partial            bool       `json:"partial"`
	SavedBy            string     `json:"saved_by"`
	SavedAt            time.Time  `json:"saved_at"`
}

// SaveRetentionScenarioInput is the write model for POST.
type SaveRetentionScenarioInput struct {
	DatabaseName       string
	SchemaName         string
	HypertableName     string
	Label              string
	RetentionDays      int
	CompressAfterDays  int
	AuditRunID         string
	Result             tsretention.SimulationResult
	Scenario           tsretention.ScenarioResult
	Currency           string
	MoneySaved         *float64
	PriceVersionID     string
	PriceEffectiveFrom *time.Time
	SavedBy            string
}

// ListRetentionScenarios returns saved scenarios for an environment (newest first).
func (s *Store) ListRetentionScenarios(ctx context.Context, environmentID string) ([]RetentionScenario, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id::text, environment_id::text, database_name, schema_name, hypertable_name, label,
  retention_days, compress_after_days, audit_run_id::text, current_bytes, bytes_after, bytes_saved,
  gb_saved::float8, currency, money_saved, price_version_id::text, price_effective_from,
  risks, ratio_confidence_low, observed_ratio, partial, saved_by, saved_at
FROM retention_scenario
WHERE environment_id=$1::uuid
ORDER BY saved_at DESC
LIMIT 500`, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]RetentionScenario, 0)
	for rows.Next() {
		item, err := scanRetentionScenario(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// SaveRetentionScenario persists one scenario and returns it.
func (s *Store) SaveRetentionScenario(ctx context.Context, environmentID string, in SaveRetentionScenarioInput) (*RetentionScenario, error) {
	risks, _ := json.Marshal(in.Scenario.Risks)
	if risks == nil {
		risks = []byte("[]")
	}
	var money any
	if in.MoneySaved != nil {
		money = *in.MoneySaved
	}
	var priceID any
	if in.PriceVersionID != "" {
		priceID = in.PriceVersionID
	}
	var runID any
	if in.AuditRunID != "" {
		runID = in.AuditRunID
	}
	var observed any
	if in.Result.ObservedRatio > 0 {
		observed = in.Result.ObservedRatio
	}
	row := s.pool.QueryRow(ctx, `
INSERT INTO retention_scenario (
  environment_id, database_name, schema_name, hypertable_name, label,
  retention_days, compress_after_days, audit_run_id,
  current_bytes, bytes_after, bytes_saved, gb_saved,
  currency, money_saved, price_version_id, price_effective_from,
  risks, ratio_confidence_low, observed_ratio, partial, saved_by
) VALUES (
  $1::uuid, $2, $3, $4, $5,
  $6, $7, NULLIF($8,'')::uuid,
  $9, $10, $11, $12,
  $13, $14, $15::uuid, $16,
  $17::jsonb, $18, $19, $20, $21
)
RETURNING id::text, environment_id::text, database_name, schema_name, hypertable_name, label,
  retention_days, compress_after_days, audit_run_id::text, current_bytes, bytes_after, bytes_saved,
  gb_saved::float8, currency, money_saved, price_version_id::text, price_effective_from,
  risks, ratio_confidence_low, observed_ratio, partial, saved_by, saved_at`,
		environmentID, in.DatabaseName, in.SchemaName, in.HypertableName, in.Label,
		in.RetentionDays, in.CompressAfterDays, runID,
		in.Result.CurrentBytes, in.Scenario.BytesAfter, in.Scenario.BytesSaved, in.Scenario.GBSaved,
		in.Currency, money, priceID, in.PriceEffectiveFrom,
		risks, in.Result.RatioConfidenceLow, observed, in.Result.Partial, in.SavedBy,
	)
	item, err := scanRetentionScenario(row)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

type retentionScanner interface {
	Scan(dest ...any) error
}

func scanRetentionScenario(row retentionScanner) (RetentionScenario, error) {
	var item RetentionScenario
	var runID, priceID *string
	var money, observed *float64
	var risks []byte
	err := row.Scan(
		&item.ID, &item.EnvironmentID, &item.DatabaseName, &item.SchemaName, &item.HypertableName, &item.Label,
		&item.RetentionDays, &item.CompressAfterDays, &runID, &item.CurrentBytes, &item.BytesAfter, &item.BytesSaved,
		&item.GBSaved, &item.Currency, &money, &priceID, &item.PriceEffectiveFrom,
		&risks, &item.RatioConfidenceLow, &observed, &item.Partial, &item.SavedBy, &item.SavedAt,
	)
	if err != nil {
		return item, err
	}
	item.AuditRunID = runID
	item.PriceVersionID = priceID
	item.MoneySaved = money
	item.ObservedRatio = observed
	item.Risks = []string{}
	if len(risks) > 0 {
		_ = json.Unmarshal(risks, &item.Risks)
	}
	return item, nil
}

// SimulationChunkRow is one chunk used to build HypertableState.
type SimulationChunkRow struct {
	ChunkSchema            string
	ChunkName              string
	RangeStart             *time.Time
	RangeEnd               *time.Time
	RangeEndInteger        *int64
	IsCompressed           bool
	TotalSizeBytes         int64
	BeforeCompressionBytes int64
	AfterCompressionBytes  int64
	DeadTuples             int64
	SeqScan                *int64
	IdxScan                *int64
	NTupIns                *int64
	NTupUpd                *int64
	NTupDel                *int64
	StatsReset             *time.Time
}

// SimulationCAGGDep is a dependent continuous aggregate for risk checks.
type SimulationCAGGDep struct {
	ViewName          string
	RefreshWindowDays int
	HasRefreshPolicy  bool
}

// HypertableSimulationInput bundles snapshot rows for one hypertable.
type HypertableSimulationInput struct {
	Found                bool
	NotApplicable        bool // no Timescale / no hypertable snapshot
	Reason               string
	CompressionEnabled   bool
	InventoryTruncated   bool
	IntegerDimensionOnly bool
	GrowthPerDayBytes    int64
	Chunks               []SimulationChunkRow
	CAGGDeps             []SimulationCAGGDep
	Policies             []HypertablePolicy
}

// LoadHypertableSimulationInput loads chunks, CAGG deps and growth for simulation.
func (s *Store) LoadHypertableSimulationInput(ctx context.Context, env, run, database, schema, name string) (*HypertableSimulationInput, error) {
	out := &HypertableSimulationInput{}
	var hasTS bool
	err := s.pool.QueryRow(ctx, `
SELECT EXISTS(
  SELECT 1 FROM timescale_version_snapshot
  WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid
)`, env, run).Scan(&hasTS)
	if err != nil {
		return nil, err
	}
	if !hasTS {
		out.NotApplicable = true
		out.Reason = "não aplicável"
		return out, nil
	}
	var compression bool
	err = s.pool.QueryRow(ctx, `
SELECT compression_enabled FROM hypertable_snapshot
WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid
  AND database_name=$3 AND schema_name=$4 AND hypertable_name=$5`,
		env, run, database, schema, name).Scan(&compression)
	if err != nil {
		out.NotApplicable = true
		out.Reason = "não aplicável"
		return out, nil
	}
	out.Found = true
	out.CompressionEnabled = compression

	_ = s.pool.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1 FROM audit_run_coverage
  WHERE audit_run_id=$1::uuid AND warning ILIKE '%truncated%'
)`, run).Scan(&out.InventoryTruncated)

	var intOnly bool
	_ = s.pool.QueryRow(ctx, `
SELECT NOT EXISTS (
  SELECT 1 FROM dimension_snapshot
  WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid
    AND database_name=$3 AND schema_name=$4 AND hypertable_name=$5
    AND (COALESCE(time_interval,'') <> '' OR COALESCE(integer_now_func,'') <> '')
) AND EXISTS (
  SELECT 1 FROM dimension_snapshot
  WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid
    AND database_name=$3 AND schema_name=$4 AND hypertable_name=$5
    AND COALESCE(integer_interval,'') <> ''
)`, env, run, database, schema, name).Scan(&intOnly)
	out.IntegerDimensionOnly = intOnly

	rows, err := s.pool.Query(ctx, `
SELECT c.chunk_schema, c.chunk_name, c.range_start, c.range_end, c.range_end_integer,
  c.is_compressed, c.total_size_bytes, c.before_compression_bytes, c.after_compression_bytes,
  COALESCE(v.n_dead_tup, 0),
  c.seq_scan, c.idx_scan, c.n_tup_ins, c.n_tup_upd, c.n_tup_del, c.stats_reset
FROM chunk_snapshot c
LEFT JOIN chunk_vacuum_sample v
  ON v.audit_run_id=c.audit_run_id AND v.database_name=c.database_name
 AND v.chunk_schema=c.chunk_schema AND v.chunk_name=c.chunk_name
WHERE c.environment_id=$1::uuid AND c.audit_run_id=$2::uuid
  AND c.database_name=$3 AND c.schema_name=$4 AND c.hypertable_name=$5
ORDER BY c.range_end NULLS LAST, c.chunk_name
LIMIT 10000`, env, run, database, schema, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var c SimulationChunkRow
		if err := rows.Scan(
			&c.ChunkSchema, &c.ChunkName, &c.RangeStart, &c.RangeEnd, &c.RangeEndInteger,
			&c.IsCompressed, &c.TotalSizeBytes, &c.BeforeCompressionBytes, &c.AfterCompressionBytes,
			&c.DeadTuples, &c.SeqScan, &c.IdxScan, &c.NTupIns, &c.NTupUpd, &c.NTupDel, &c.StatsReset,
		); err != nil {
			return nil, err
		}
		out.Chunks = append(out.Chunks, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	caggRows, err := s.pool.Query(ctx, `
SELECT c.view_name,
  EXISTS(
    SELECT 1 FROM policy_snapshot p
    WHERE p.audit_run_id=c.audit_run_id AND p.database_name=c.database_name
      AND p.policy_type='refresh'
      AND ((p.hypertable_schema=c.schema_name AND p.hypertable_name=c.view_name)
        OR (p.hypertable_schema=c.materialization_schema AND p.hypertable_name=c.materialization_hypertable))
  ) AS has_refresh,
  COALESCE((
    SELECT p.config_json FROM policy_snapshot p
    WHERE p.audit_run_id=c.audit_run_id AND p.database_name=c.database_name
      AND p.policy_type='refresh'
      AND ((p.hypertable_schema=c.schema_name AND p.hypertable_name=c.view_name)
        OR (p.hypertable_schema=c.materialization_schema AND p.hypertable_name=c.materialization_hypertable))
    LIMIT 1
  ), '') AS config_json
FROM continuous_aggregate_snapshot c
WHERE c.environment_id=$1::uuid AND c.audit_run_id=$2::uuid
  AND c.database_name=$3
  AND ((c.source_hypertable_schema=$4 AND c.source_hypertable_name=$5)
    OR (c.schema_name=$4 AND c.view_name=$5))`, env, run, database, schema, name)
	if err != nil {
		return nil, err
	}
	defer caggRows.Close()
	for caggRows.Next() {
		var dep SimulationCAGGDep
		var cfg string
		if err := caggRows.Scan(&dep.ViewName, &dep.HasRefreshPolicy, &cfg); err != nil {
			return nil, err
		}
		dep.RefreshWindowDays = refreshWindowDaysFromConfig(cfg)
		out.CAGGDeps = append(out.CAGGDeps, dep)
	}
	if err := caggRows.Err(); err != nil {
		return nil, err
	}

	policies, _, err := s.ListHypertablePolicies(ctx, env, run, database, schema, name, 100, 0)
	if err == nil {
		out.Policies = policies
	}
	out.GrowthPerDayBytes = s.hypertableGrowthPerDay(ctx, env, run, database, schema, name)
	return out, nil
}

func refreshWindowDaysFromConfig(cfg string) int {
	if strings.TrimSpace(cfg) == "" {
		return 0
	}
	var raw map[string]any
	if json.Unmarshal([]byte(cfg), &raw) != nil {
		return 0
	}
	start, _ := raw["start_offset"].(string)
	d := parseLooseInterval(start)
	if d <= 0 {
		return 0
	}
	return int(d / (24 * time.Hour))
}

func parseLooseInterval(v string) time.Duration {
	v = strings.TrimSpace(strings.ToLower(v))
	if v == "" {
		return 0
	}
	// Accept "120 days", "120 day", "2880:00:00" roughly via hours/days tokens.
	fields := strings.Fields(v)
	if len(fields) >= 2 {
		var n float64
		if _, err := fmt.Sscanf(fields[0], "%f", &n); err == nil {
			unit := fields[1]
			switch {
			case strings.HasPrefix(unit, "day"):
				return time.Duration(n) * 24 * time.Hour
			case strings.HasPrefix(unit, "hour"):
				return time.Duration(n) * time.Hour
			case strings.HasPrefix(unit, "week"):
				return time.Duration(n) * 7 * 24 * time.Hour
			}
		}
	}
	if d, err := time.ParseDuration(strings.ReplaceAll(v, " ", "")); err == nil {
		return d
	}
	return 0
}

func (s *Store) hypertableGrowthPerDay(ctx context.Context, env, run, database, schema, name string) int64 {
	// PairComparable rule of maintenance.growth_trend: two complete runs ≤ 14 days apart.
	var curAt time.Time
	var curBytes int64
	err := s.pool.QueryRow(ctx, `
SELECT r.finished_at, h.total_size_bytes
FROM audit_run r
JOIN hypertable_snapshot h ON h.audit_run_id=r.id
WHERE r.id=$1::uuid AND r.environment_id=$2::uuid AND r.status='success'
  AND h.database_name=$3 AND h.schema_name=$4 AND h.hypertable_name=$5`,
		run, env, database, schema, name).Scan(&curAt, &curBytes)
	if err != nil || curAt.IsZero() {
		return 0
	}
	var prevAt time.Time
	var prevBytes int64
	err = s.pool.QueryRow(ctx, `
SELECT r.finished_at, h.total_size_bytes
FROM audit_run r
JOIN hypertable_snapshot h ON h.audit_run_id=r.id AND h.environment_id=r.environment_id
WHERE r.environment_id=$1::uuid AND r.status='success' AND r.finished_at < $2
  AND r.finished_at >= $2 - interval '14 days'
  AND h.database_name=$3 AND h.schema_name=$4 AND h.hypertable_name=$5
ORDER BY r.finished_at DESC
LIMIT 1`, env, curAt, database, schema, name).Scan(&prevAt, &prevBytes)
	if err != nil || prevAt.IsZero() || !curAt.After(prevAt) {
		return 0
	}
	days := curAt.Sub(prevAt).Hours() / 24
	if days < 0.5 {
		return 0
	}
	delta := curBytes - prevBytes
	if delta < 0 {
		return 0
	}
	return int64(float64(delta) / days)
}

// ListChunkScanObservations loads scan counters for unscanned-chunks (RF-07).
func (s *Store) ListChunkScanObservations(ctx context.Context, env, run, database, schema, name string) ([]tsretention.ChunkScanObservation, error) {
	rows, err := s.pool.Query(ctx, `
SELECT chunk_schema || '.' || chunk_name, seq_scan, idx_scan, n_tup_ins, n_tup_upd, n_tup_del, stats_reset
FROM chunk_snapshot
WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid
  AND database_name=$3 AND schema_name=$4 AND hypertable_name=$5
ORDER BY chunk_name`, env, run, database, schema, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]tsretention.ChunkScanObservation, 0)
	for rows.Next() {
		var o tsretention.ChunkScanObservation
		if err := rows.Scan(&o.Name, &o.SeqScan, &o.IdxScan, &o.NTupIns, &o.NTupUpd, &o.NTupDel, &o.StatsReset); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// PreviousCompleteRunID returns the prior complete success run for the environment, if any.
func (s *Store) PreviousCompleteRunID(ctx context.Context, env, run string) (string, error) {
	var prev string
	err := s.pool.QueryRow(ctx, `
SELECT prev.id::text
FROM audit_run cur
JOIN audit_run prev ON prev.environment_id=cur.environment_id
  AND prev.status='success' AND prev.finished_at < cur.finished_at
  AND prev.finished_at >= cur.finished_at - interval '14 days'
WHERE cur.id=$1::uuid AND cur.environment_id=$2::uuid AND cur.status IN ('success','partial_success')
ORDER BY prev.finished_at DESC
LIMIT 1`, run, env).Scan(&prev)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return prev, nil
}

// ApplyStoragePrice converts GB saved to monthly storage money when a price exists.
func ApplyStoragePrice(gbSaved float64, price *CostPriceVersion) (currency string, money *float64, priceID string, effective *time.Time) {
	if price == nil || price.StorageGBMonth < 0 {
		return "", nil, "", nil
	}
	amt := costlens.RoundMoney(gbSaved * price.StorageGBMonth)
	return price.Currency, &amt, price.ID, &price.EffectiveFrom
}

// FormatRetentionScenariosCSV emits one row per scenario + # export_complete.
func FormatRetentionScenariosCSV(items []RetentionScenario) string {
	var b strings.Builder
	b.WriteString("database,schema,hypertable,label,retention_days,compress_after_days,gb_saved,currency,money_saved,risks,partial,saved_at\n")
	for _, it := range items {
		money := ""
		if it.MoneySaved != nil {
			money = fmt.Sprintf("%.2f", *it.MoneySaved)
		}
		risks := strings.Join(it.Risks, "|")
		fmt.Fprintf(&b, "%s,%s,%s,%s,%d,%d,%.4f,%s,%s,%s,%t,%s\n",
			csvEscape(it.DatabaseName), csvEscape(it.SchemaName), csvEscape(it.HypertableName),
			csvEscape(it.Label), it.RetentionDays, it.CompressAfterDays, it.GBSaved,
			csvEscape(it.Currency), money, csvEscape(risks), it.Partial,
			it.SavedAt.UTC().Format(time.RFC3339),
		)
	}
	fmt.Fprintf(&b, "# export_complete,%d\n", len(items))
	return b.String()
}

func csvEscape(v string) string {
	if strings.ContainsAny(v, ",\"\n") {
		return `"` + strings.ReplaceAll(v, `"`, `""`) + `"`
	}
	return v
}
