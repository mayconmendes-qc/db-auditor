package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/osmendes/db-auditor/internal/costlens"
	"github.com/osmendes/db-auditor/internal/digest"
	"github.com/osmendes/db-auditor/internal/observability"
)

// CostRankingFilter selects ranking rows.
type CostRankingFilter struct {
	EnvironmentID string
	AuditRunID    string // empty = latest complete
	Database      string
	Schema        string
	Type          string // storage|unused_index|bloat|query|""
}

// CostRankingResponse is GET /cost/ranking.
type CostRankingResponse struct {
	costlens.Ranking
	AuditRunID   string `json:"audit_run_id,omitempty"`
	Completeness string `json:"completeness,omitempty"`
}

type costOpenFinding struct {
	ID          string
	FindingType string
	Database    string
	Schema      string
	ObjectName  string
	SizeBytes   int64
	FirstSeenAt time.Time
	LastSeenAt  time.Time
	StatsReset  *time.Time
}

// ComputeCostRanking builds the top waste ranking from the snapshot store (RNF-03).
func (s *Store) ComputeCostRanking(ctx context.Context, f CostRankingFilter) (*CostRankingResponse, error) {
	runID := f.AuditRunID
	var started time.Time
	var err error
	if runID == "" {
		runID, started, err = s.latestCompleteRunID(ctx, f.EnvironmentID)
		if err != nil {
			return nil, err
		}
		if runID == "" {
			r := costlens.BuildRanking(nil, 10, nil, "sem coleta completa")
			return &CostRankingResponse{Ranking: r, Completeness: "empty"}, nil
		}
	} else {
		_ = s.pool.QueryRow(ctx, `SELECT started_at FROM audit_run WHERE id=$1::uuid AND environment_id=$2::uuid`, runID, f.EnvironmentID).Scan(&started)
	}

	priceRow, err := s.CurrentCostPrice(ctx, f.EnvironmentID)
	if err != nil {
		return nil, err
	}
	price := ToCostlensPrice(priceRow)
	priced := price != nil
	observability.DefaultMetrics.IncCostCalculation(priced)

	items := make([]costlens.CostResult, 0, 256)

	// Storage by table (RF-02).
	tableRows, err := s.pool.Query(ctx, `
SELECT database_name, schema_name, table_name, total_size_bytes, row_estimate
FROM table_snapshot
WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid
  AND schema_name <> '_timescaledb_internal'
  AND ($3 = '' OR database_name = $3)
  AND ($4 = '' OR schema_name = $4)`, f.EnvironmentID, runID, f.Database, f.Schema)
	if err != nil {
		return nil, err
	}
	type tableRow struct {
		db, schema, name string
		size, rows       int64
	}
	tables := []tableRow{}
	for tableRows.Next() {
		var t tableRow
		if err := tableRows.Scan(&t.db, &t.schema, &t.name, &t.size, &t.rows); err != nil {
			tableRows.Close()
			return nil, err
		}
		tables = append(tables, t)
	}
	tableRows.Close()
	if err := tableRows.Err(); err != nil {
		return nil, err
	}

	if f.Type == "" || f.Type == "storage" {
		for _, t := range tables {
			key := t.db + "." + t.schema + "." + t.name
			item := costlens.StorageItem{Kind: "table", Database: t.db, Schema: t.schema, Name: t.name, ObjectKey: key, SizeBytes: t.size}
			items = append(items, costlens.EstimateStorage(item, price))
		}
	}

	// Unused index waste from open index.unused findings (RF-03).
	if f.Type == "" || f.Type == "unused_index" {
		unused, err := s.listOpenUnusedIndexes(ctx, f.EnvironmentID, runID, f.Database, f.Schema)
		if err != nil {
			return nil, err
		}
		for _, u := range unused {
			if u.SizeBytes < costlens.UnusedIndexMinBytes {
				continue
			}
			windowDays := 0.0
			if !u.FirstSeenAt.IsZero() && u.LastSeenAt.After(u.FirstSeenAt) {
				windowDays = u.LastSeenAt.Sub(u.FirstSeenAt).Hours() / 24
			}
			if windowDays < float64(costlens.UnusedIndexMinWindowDay) {
				continue
			}
			key := u.Database + "." + u.Schema + "." + u.ObjectName
			items = append(items, costlens.EstimateUnusedIndexWaste(u.SizeBytes, key, u.Database, u.Schema, u.ObjectName, u.ID, price))
		}
	}

	// Bloat estimated (RF-04).
	if f.Type == "" || f.Type == "bloat" {
		colStats, err := s.columnStatsByTable(ctx, f.EnvironmentID, runID)
		if err != nil {
			return nil, err
		}
		for _, t := range tables {
			cols := colStats[t.db+"."+t.schema+"."+t.name]
			est := costlens.EstimateBloat(t.size, t.rows, cols)
			if !costlens.MeetsBloatRule(est) {
				continue
			}
			key := t.db + "." + t.schema + "." + t.name
			items = append(items, costlens.EstimateBloatCost(est, key, t.db, t.schema, t.name, price))
		}
	}

	// Query cost via digest window (RF-05); optional when no comparable window.
	if f.Type == "" || f.Type == "query" {
		qItems, _ := s.computeQueryCostItems(ctx, f.EnvironmentID, price)
		for _, q := range qItems {
			if f.Database != "" && q.Database != f.Database {
				continue
			}
			items = append(items, q)
		}
	}

	ranking := costlens.BuildRanking(items, 10, price, "")
	return &CostRankingResponse{
		Ranking:      ranking,
		AuditRunID:   runID,
		Completeness: "complete",
	}, nil
}

func (s *Store) listOpenUnusedIndexes(ctx context.Context, env, runID, database, schema string) ([]costOpenFinding, error) {
	rows, err := s.pool.Query(ctx, `
SELECT f.id, f.finding_type, f.database_name, f.schema_name, f.object_name,
       COALESCE(i.size_bytes, 0), f.first_seen_at, f.last_seen_at, i.stats_reset
FROM finding f
LEFT JOIN index_snapshot i
  ON i.audit_run_id = $2::uuid
 AND i.environment_id = f.environment_id
 AND i.database_name = f.database_name
 AND i.schema_name = f.schema_name
 AND i.index_name = f.object_name
WHERE f.environment_id=$1::uuid
  AND f.status = 'open'
  AND f.finding_type = 'index.unused'
  AND ($3 = '' OR f.database_name = $3)
  AND ($4 = '' OR f.schema_name = $4)`, env, runID, database, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []costOpenFinding{}
	for rows.Next() {
		var u costOpenFinding
		if err := rows.Scan(&u.ID, &u.FindingType, &u.Database, &u.Schema, &u.ObjectName, &u.SizeBytes, &u.FirstSeenAt, &u.LastSeenAt, &u.StatsReset); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) columnStatsByTable(ctx context.Context, env, runID string) (map[string][]costlens.ColumnWidthStat, error) {
	rows, err := s.pool.Query(ctx, `
SELECT database_name, schema_name, table_name, average_width, null_fraction
FROM column_stat_snapshot
WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid`, env, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]costlens.ColumnWidthStat{}
	for rows.Next() {
		var db, schema, table string
		var avg int
		var nf float64
		if err := rows.Scan(&db, &schema, &table, &avg, &nf); err != nil {
			return nil, err
		}
		key := db + "." + schema + "." + table
		out[key] = append(out[key], costlens.ColumnWidthStat{AverageWidth: avg, NullFraction: nf})
	}
	return out, rows.Err()
}

func (s *Store) computeQueryCostItems(ctx context.Context, env string, price *costlens.PriceVersion) ([]costlens.CostResult, error) {
	points, workload, err := s.loadDigestWindowInputs(ctx, env, 90)
	if err != nil {
		return nil, err
	}
	win := digest.ResolveWindow(points, workload, 90*24*time.Hour)
	status := costlens.ValidateQueryWindow(win)
	if !status.OK {
		return []costlens.CostResult{{
			Type: "query", Reason: status.Reason, Label: status.Reason, Confidence: "none",
		}}, nil
	}
	var totalBlocks int64
	for _, d := range win.Deltas {
		if d.DeltaBlocksRead > 0 {
			totalBlocks += d.DeltaBlocksRead
		}
	}
	out := []costlens.CostResult{}
	for _, d := range win.Deltas {
		if d.DeltaCalls <= 0 && d.DeltaTotalMS <= 0 {
			continue
		}
		share := 0.0
		if win.TotalTimeMS > 0 {
			share = d.DeltaTotalMS / win.TotalTimeMS
		}
		if !costlens.MeetsQueryExpensiveRule(share, d.DeltaCalls) {
			continue
		}
		in := costlens.QueryCostInput{
			Fingerprint: d.Fingerprint, Database: d.DatabaseName,
			DeltaCalls: d.DeltaCalls, DeltaTotalMS: d.DeltaTotalMS, DeltaBlocksRead: d.DeltaBlocksRead,
			TotalTimeMS: win.TotalTimeMS, TotalBlocksRead: totalBlocks,
		}
		out = append(out, costlens.EstimateQueryCost(in, price, status))
	}
	return out, nil
}

func (s *Store) loadDigestWindowInputs(ctx context.Context, env string, windowDays int) ([]digest.CollectionPoint, []digest.WorkloadPoint, error) {
	to := time.Now().UTC()
	from := to.AddDate(0, 0, -windowDays)
	points, err := s.listCollectionEdges(ctx, env, from, to)
	if err != nil {
		return nil, nil, err
	}
	runIDs := make([]string, 0, len(points))
	for _, p := range points {
		runIDs = append(runIDs, p.AuditRunID)
	}
	workload, err := s.listWorkloadForRuns(ctx, env, runIDs)
	if err != nil {
		return points, nil, nil
	}
	return points, workload, nil
}

// ListCostSeries returns estimate snapshots in [from,to] (max 90 days).
func (s *Store) ListCostSeries(ctx context.Context, environmentID string, from, to time.Time) ([]costlens.SeriesPoint, error) {
	if to.Before(from) {
		return nil, fmt.Errorf("janela inválida")
	}
	if to.Sub(from) > 90*24*time.Hour+time.Hour {
		return nil, fmt.Errorf("janela máxima de 90 dias")
	}
	rows, err := s.pool.Query(ctx, `
SELECT e.audit_run_id::text, e.collected_at, e.currency, e.storage_month, e.unused_index_month,
       e.bloat_month, e.query_month, e.priced, e.confidence, e.reason
FROM cost_estimate_snapshot e
WHERE e.environment_id=$1::uuid AND e.collected_at >= $2 AND e.collected_at <= $3
ORDER BY e.collected_at ASC`, environmentID, from.UTC(), to.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []costlens.SeriesPoint{}
	for rows.Next() {
		var p costlens.SeriesPoint
		var at time.Time
		if err := rows.Scan(&p.AuditRunID, &at, &p.Currency, &p.StorageMonth, &p.UnusedIndexMonth,
			&p.BloatMonth, &p.QueryMonth, &p.Priced, &p.Confidence, &p.Reason); err != nil {
			return nil, err
		}
		p.Completeness = "complete"
		p.At = at.UTC().Format(time.RFC3339)
		out = append(out, p)
	}
	return out, rows.Err()
}

// SaveCostEstimateSnapshot persists totals after a complete analysis.
func (s *Store) SaveCostEstimateSnapshot(ctx context.Context, environmentID, auditRunID string, ranking *CostRankingResponse, price *CostPriceVersion) error {
	var priceID *string
	currency := ""
	priced := false
	conf := "none"
	reason := ""
	var storage, unused, bloat, query float64
	if ranking != nil {
		storage = ranking.Totals.Storage
		unused = ranking.Totals.UnusedIndex
		bloat = ranking.Totals.Bloat
		query = ranking.Totals.Query
		priced = ranking.Priced
		currency = ranking.Currency
		if priced {
			conf = "medium"
		} else {
			reason = costlens.ReasonNoPrice
		}
	}
	if price != nil {
		priceID = &price.ID
		currency = price.Currency
	}
	_, err := s.pool.Exec(ctx, `
INSERT INTO cost_estimate_snapshot (
  environment_id, audit_run_id, price_version_id, currency,
  storage_month, unused_index_month, bloat_month, query_month,
  priced, confidence, reason, collected_at)
VALUES ($1::uuid,$2::uuid,$3::uuid,$4,$5,$6,$7,$8,$9,$10,$11,now())
ON CONFLICT (environment_id, audit_run_id) DO UPDATE SET
  price_version_id = EXCLUDED.price_version_id,
  currency = EXCLUDED.currency,
  storage_month = EXCLUDED.storage_month,
  unused_index_month = EXCLUDED.unused_index_month,
  bloat_month = EXCLUDED.bloat_month,
  query_month = EXCLUDED.query_month,
  priced = EXCLUDED.priced,
  confidence = EXCLUDED.confidence,
  reason = EXCLUDED.reason,
  collected_at = EXCLUDED.collected_at`,
		environmentID, auditRunID, priceID, currency, storage, unused, bloat, query, priced, conf, reason)
	return err
}

// FormatCostRankingCSV emits one row per ranking item + # export_complete.
func FormatCostRankingCSV(ranking *CostRankingResponse) string {
	var b strings.Builder
	b.WriteString("type,object_key,database,schema,object,size_bytes,month_amount,currency,priced,reason,formula,confidence,price_version,price_effective,fingerprint\n")
	items := []costlens.CostResult{}
	if ranking != nil {
		items = ranking.Items
	}
	for _, it := range items {
		amt := ""
		if it.MonthAmount != nil {
			amt = fmt.Sprintf("%.2f", *it.MonthAmount)
		}
		// Never emit SQL text; fingerprint is already normalized.
		fmt.Fprintf(&b, "%s,%s,%s,%s,%s,%d,%s,%s,%t,%s,%s,%s,%d,%s,%s\n",
			csvEsc(it.Type), csvEsc(it.ObjectKey), csvEsc(it.Database), csvEsc(it.Schema), csvEsc(it.Object),
			it.SizeBytes, amt, csvEsc(it.Currency), it.Priced, csvEsc(it.Reason), csvEsc(it.Formula),
			csvEsc(it.Confidence), it.PriceVersion, csvEsc(it.PriceEffective), csvEsc(it.Fingerprint))
	}
	b.WriteString("# export_complete\n")
	return b.String()
}

func csvEsc(s string) string {
	if strings.ContainsAny(s, ",\"\n\r") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}

// TableStorageCostMap returns object_key → cost label for inventory column.
func (s *Store) TableStorageCostMap(ctx context.Context, environmentID string) (map[string]costlens.CostResult, error) {
	runID, _, err := s.latestCompleteRunID(ctx, environmentID)
	if err != nil || runID == "" {
		return map[string]costlens.CostResult{}, err
	}
	priceRow, err := s.CurrentCostPrice(ctx, environmentID)
	if err != nil {
		return nil, err
	}
	price := ToCostlensPrice(priceRow)
	rows, err := s.pool.Query(ctx, `
SELECT database_name, schema_name, table_name, total_size_bytes
FROM table_snapshot
WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid`, environmentID, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]costlens.CostResult{}
	for rows.Next() {
		var db, schema, name string
		var size int64
		if err := rows.Scan(&db, &schema, &name, &size); err != nil {
			return nil, err
		}
		key := db + "." + schema + "." + name
		out[key] = costlens.EstimateStorage(costlens.StorageItem{
			Kind: "table", Database: db, Schema: schema, Name: name, ObjectKey: key, SizeBytes: size,
		}, price)
	}
	return out, rows.Err()
}

// PersistCostEstimatesForRun stores series totals after a successful complete analysis (L4).
func (s *Store) PersistCostEstimatesForRun(ctx context.Context, environmentID, auditRunID string) error {
	cov, err := s.GetSnapshotCompleteness(ctx, environmentID, auditRunID)
	if err != nil {
		return err
	}
	if cov == nil || cov.Completeness != "complete" {
		return nil // lacuna: never persist partial runs
	}
	ranking, err := s.ComputeCostRanking(ctx, CostRankingFilter{EnvironmentID: environmentID, AuditRunID: auditRunID})
	if err != nil {
		return err
	}
	price, err := s.CurrentCostPrice(ctx, environmentID)
	if err != nil {
		return err
	}
	return s.SaveCostEstimateSnapshot(ctx, environmentID, auditRunID, ranking, price)
}
