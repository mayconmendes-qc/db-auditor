package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

var ErrMeasurementRun = errors.New("measurement run is outside finding scope")

type ActionMeasurement struct {
	ID                 string    `json:"id"`
	FindingID          string    `json:"finding_id"`
	BeforeRunID        string    `json:"before_run_id"`
	AfterRunID         string    `json:"after_run_id"`
	Metric             string    `json:"metric"`
	BeforeValue        *int64    `json:"before_value"`
	AfterValue         *int64    `json:"after_value"`
	Comparable         bool      `json:"comparable"`
	ComparisonNote     string    `json:"comparison_note"`
	Hypothesis         string    `json:"hypothesis"`
	WindowNote         string    `json:"window_note"`
	WorkloadComparable bool      `json:"workload_comparable"`
	RecordedBy         string    `json:"recorded_by"`
	RecordedAt         time.Time `json:"recorded_at"`
}

type TrackedAction struct {
	FindingID             string              `json:"finding_id"`
	EnvironmentID         string              `json:"environment_id"`
	Title                 string              `json:"title"`
	Severity              string              `json:"severity"`
	Status                string              `json:"status"`
	Owner                 string              `json:"owner"`
	Result                string              `json:"result"`
	Recurrences           int                 `json:"recurrences"`
	DueAt                 *time.Time          `json:"due_at,omitempty"`
	LatestMeasurement     *ActionMeasurement  `json:"latest_measurement,omitempty"`
	Measurements          []ActionMeasurement `json:"measurements"`
	PotentialReclaimBytes *int64              `json:"potential_reclaim_bytes,omitempty"`
	EstimateNote          string              `json:"estimate_note"`
}

func (s *Store) ListTrackedActions(ctx context.Context, env string) ([]TrackedAction, error) {
	return s.listTrackedActions(ctx, env, 100, 0)
}

func (s *Store) ListTrackedActionsPage(ctx context.Context, env string, limit, offset int) ([]TrackedAction, int, error) {
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM finding_action a JOIN finding f ON f.id=a.finding_id WHERE f.environment_id=$1::uuid`, env).Scan(&total); err != nil {
		return nil, 0, err
	}
	items, err := s.listTrackedActions(ctx, env, limit, offset)
	return items, total, err
}

func (s *Store) listTrackedActions(ctx context.Context, env string, limit, offset int) ([]TrackedAction, error) {
	rows, err := s.pool.Query(ctx, `SELECT f.id::text,f.environment_id::text,f.title,f.severity,a.status,a.owner_name,a.result_note,f.recurrence_count,f.due_at,f.finding_type,
CASE WHEN f.finding_type='index.unused' THEN (SELECT i.size_bytes FROM index_snapshot i WHERE i.audit_run_id=f.audit_run_id AND i.database_name=f.database_name AND i.schema_name=f.schema_name AND i.index_name=f.object_name LIMIT 1)
WHEN f.finding_type='vacuum.high_dead_tuples' THEN (SELECT LEAST(t.data_size_bytes::numeric,TRUNC(t.data_size_bytes::numeric*t.n_dead_tup::numeric/GREATEST(t.n_live_tup::numeric+t.n_dead_tup::numeric,1)))::bigint FROM table_snapshot t WHERE t.audit_run_id=f.audit_run_id AND t.database_name=f.database_name AND t.schema_name=f.schema_name AND t.table_name=f.object_name LIMIT 1) END
FROM finding_action a JOIN finding f ON f.id=a.finding_id WHERE f.environment_id=$1::uuid ORDER BY a.updated_at DESC,f.id DESC LIMIT $2 OFFSET $3`, env, limit, offset)
	if err != nil {
		return nil, err
	}
	out := []TrackedAction{}
	for rows.Next() {
		var item TrackedAction
		var findingType string
		if err = rows.Scan(&item.FindingID, &item.EnvironmentID, &item.Title, &item.Severity, &item.Status, &item.Owner, &item.Result, &item.Recurrences, &item.DueAt, &findingType, &item.PotentialReclaimBytes); err != nil {
			rows.Close()
			return nil, err
		}
		if item.PotentialReclaimBytes != nil && *item.PotentialReclaimBytes > 0 {
			if findingType == "vacuum.high_dead_tuples" {
				item.EstimateNote = "Proxy de bytes associados a tuplas mortas, calculado com proporção estimada e tamanho da tabela. Não representa espaço recuperável no sistema de arquivos; confirme com medições e manutenção controlada."
			} else {
				item.EstimateNote = "Limite superior de espaço candidato: tamanho observado do índice. Só seria recuperável após revisão e remoção externa; uso futuro e dependências podem impedir a ação."
			}
		} else {
			item.PotentialReclaimBytes = nil
			item.EstimateNote = "Sem base suficiente para estimar impacto numérico."
		}
		out = append(out, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}
	ids := make([]string, len(out))
	positions := make(map[string]int, len(out))
	for i := range out {
		ids[i] = out[i].FindingID
		positions[ids[i]] = i
		out[i].Measurements = []ActionMeasurement{}
	}
	latest, err := s.pool.Query(ctx, `SELECT id::text,finding_id::text,before_run_id::text,after_run_id::text,metric,before_value,after_value,comparable,comparison_note,hypothesis,window_note,workload_comparable,recorded_by,recorded_at FROM (
SELECT m.*,row_number() OVER (PARTITION BY finding_id ORDER BY recorded_at DESC,id DESC) AS position
FROM finding_action_measurement m WHERE finding_id=ANY($1::uuid[])) m
WHERE position<=100 ORDER BY finding_id,recorded_at DESC,id DESC`, ids)
	if err != nil {
		return nil, err
	}
	for latest.Next() {
		var item ActionMeasurement
		if err = latest.Scan(&item.ID, &item.FindingID, &item.BeforeRunID, &item.AfterRunID, &item.Metric, &item.BeforeValue, &item.AfterValue, &item.Comparable, &item.ComparisonNote, &item.Hypothesis, &item.WindowNote, &item.WorkloadComparable, &item.RecordedBy, &item.RecordedAt); err != nil {
			latest.Close()
			return nil, err
		}
		position := positions[item.FindingID]
		out[position].Measurements = append(out[position].Measurements, item)
		if out[position].LatestMeasurement == nil {
			copyOfLatest := item
			out[position].LatestMeasurement = &copyOfLatest
		}
	}
	err = latest.Err()
	latest.Close()
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) ListActionMeasurements(ctx context.Context, findingID string) ([]ActionMeasurement, error) {
	return s.listActionMeasurements(ctx, findingID, 100, 0)
}

func (s *Store) ListActionMeasurementsPage(ctx context.Context, findingID string, limit, offset int) ([]ActionMeasurement, int, error) {
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM finding_action_measurement WHERE finding_id=$1::uuid`, findingID).Scan(&total); err != nil {
		return nil, 0, err
	}
	items, err := s.listActionMeasurements(ctx, findingID, limit, offset)
	return items, total, err
}

func (s *Store) listActionMeasurements(ctx context.Context, findingID string, limit, offset int) ([]ActionMeasurement, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text,finding_id::text,before_run_id::text,after_run_id::text,metric,before_value,after_value,comparable,comparison_note,hypothesis,window_note,workload_comparable,recorded_by,recorded_at FROM finding_action_measurement WHERE finding_id=$1::uuid ORDER BY recorded_at DESC,id DESC LIMIT $2 OFFSET $3`, findingID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ActionMeasurement{}
	for rows.Next() {
		var item ActionMeasurement
		if err = rows.Scan(&item.ID, &item.FindingID, &item.BeforeRunID, &item.AfterRunID, &item.Metric, &item.BeforeValue, &item.AfterValue, &item.Comparable, &item.ComparisonNote, &item.Hypothesis, &item.WindowNote, &item.WorkloadComparable, &item.RecordedBy, &item.RecordedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) RecordActionMeasurement(ctx context.Context, findingID, before, after, metric, hypothesis, window string, workloadComparable bool, actor string) (*ActionMeasurement, error) {
	var env, db, schema, table, fingerprint string
	err := s.pool.QueryRow(ctx, `SELECT environment_id::text,database_name,schema_name,object_name,COALESCE(evidence->>'query_fingerprint','') FROM finding WHERE id=$1::uuid`, findingID).Scan(&env, &db, &schema, &table, &fingerprint)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrActionNotFound
	}
	if err != nil {
		return nil, err
	}
	queryMetric := metric == "query_mean_latency_us" || metric == "query_reads_per_1000_calls"
	if queryMetric && fingerprint == "" {
		return nil, ErrMeasurementRun
	}
	type runMeta struct {
		started                           time.Time
		profile, collector, rules, status string
	}
	getMeta := func(id string) (runMeta, error) {
		var x runMeta
		e := s.pool.QueryRow(ctx, `SELECT r.started_at,r.profile,r.collector_version,COALESCE(NULLIF(a.rule_manifest_hash,''),a.analyzer_version,''),r.status FROM audit_run r LEFT JOIN analysis_run a ON a.audit_run_id=r.id WHERE r.id=$1::uuid AND r.environment_id=$2::uuid`, id, env).Scan(&x.started, &x.profile, &x.collector, &x.rules, &x.status)
		return x, e
	}
	b, err := getMeta(before)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrMeasurementRun
	}
	if err != nil {
		return nil, err
	}
	a, err := getMeta(after)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrMeasurementRun
	}
	if err != nil {
		return nil, err
	}
	if !b.started.Before(a.started) {
		return nil, ErrMeasurementRun
	}
	value := func(run string) (*int64, *time.Time, error) {
		var n int64
		var e error
		var reset *time.Time
		switch metric {
		case "finding_observed":
			e = s.pool.QueryRow(ctx, `SELECT count(*) FROM finding_event WHERE finding_id=$1::uuid AND audit_run_id=$2::uuid AND event_type='observed'`, findingID, run).Scan(&n)
		case "table_size_bytes":
			e = s.pool.QueryRow(ctx, `SELECT total_size_bytes FROM table_snapshot WHERE audit_run_id=$1::uuid AND database_name=$2 AND schema_name=$3 AND table_name=$4`, run, db, schema, table).Scan(&n)
		case "query_mean_latency_us":
			e = s.pool.QueryRow(ctx, `SELECT ROUND(mean_exec_time_ms*1000)::bigint,stats_reset FROM workload_snapshot WHERE audit_run_id=$1::uuid AND database_name=$2 AND query_fingerprint=$3`, run, db, fingerprint).Scan(&n, &reset)
		case "query_reads_per_1000_calls":
			e = s.pool.QueryRow(ctx, `SELECT ROUND(shared_blocks_read::numeric*1000/GREATEST(calls,1))::bigint,stats_reset FROM workload_snapshot WHERE audit_run_id=$1::uuid AND database_name=$2 AND query_fingerprint=$3`, run, db, fingerprint).Scan(&n, &reset)
		default:
			return nil, nil, ErrMeasurementRun
		}
		if errors.Is(e, pgx.ErrNoRows) {
			return nil, nil, nil
		}
		if e != nil {
			return nil, nil, e
		}
		return &n, reset, nil
	}
	beforeValue, beforeReset, err := value(before)
	if err != nil {
		return nil, err
	}
	afterValue, afterReset, err := value(after)
	if err != nil {
		return nil, err
	}
	beforeCoverage, err := s.GetSnapshotCompleteness(ctx, env, before)
	if err != nil {
		return nil, err
	}
	afterCoverage, err := s.GetSnapshotCompleteness(ctx, env, after)
	if err != nil {
		return nil, err
	}
	workloadResetComparable := !queryMetric || beforeReset != nil && afterReset != nil && beforeReset.Equal(*afterReset)
	comparable := workloadComparable && workloadResetComparable && beforeValue != nil && afterValue != nil && b.status == "success" && a.status == "success" && b.profile == a.profile && b.collector == a.collector && b.rules == a.rules && b.rules != "" && beforeCoverage != nil && afterCoverage != nil && beforeCoverage.Completeness == "complete" && afterCoverage.Completeness == "complete" && beforeCoverage.AnalysisStatus != nil && afterCoverage.AnalysisStatus != nil && *beforeCoverage.AnalysisStatus == "success" && *afterCoverage.AnalysisStatus == "success"
	note := "Comparação indisponível: cobertura, análise, objeto, perfil, versão, reset de estatísticas ou carga não confirmada como semelhante."
	if comparable {
		note = "Métrica observada em runs tecnicamente comparáveis, com carga declarada semelhante pelo operador; a diferença não comprova causalidade da ação."
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var id string
	err = tx.QueryRow(ctx, `INSERT INTO finding_action_measurement(finding_id,before_run_id,after_run_id,metric,before_value,after_value,comparable,comparison_note,hypothesis,window_note,workload_comparable,recorded_by) VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5,$6,$7,$8,$9,$10,$11,$12) ON CONFLICT(finding_id,before_run_id,after_run_id,metric) DO UPDATE SET before_value=EXCLUDED.before_value,after_value=EXCLUDED.after_value,comparable=EXCLUDED.comparable,comparison_note=EXCLUDED.comparison_note,hypothesis=EXCLUDED.hypothesis,window_note=EXCLUDED.window_note,workload_comparable=EXCLUDED.workload_comparable,recorded_by=EXCLUDED.recorded_by,recorded_at=now() RETURNING id::text`, findingID, before, after, metric, beforeValue, afterValue, comparable, note, hypothesis, window, workloadComparable, actor).Scan(&id)
	if err != nil {
		return nil, err
	}
	if comparable && metric == "finding_observed" && *afterValue > 0 {
		var owner, justification, result string
		err = tx.QueryRow(ctx, `UPDATE finding_action SET status='in_review',updated_by=$2,updated_at=now() WHERE finding_id=$1::uuid AND status IN ('validated','executed_externally') RETURNING owner_name,justification,result_note`, findingID, actor).Scan(&owner, &justification, &result)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		if err == nil {
			justification += " | Reaberta: achado observado após medição comparável."
			if _, err = tx.Exec(ctx, `INSERT INTO finding_action_event(finding_id,status,owner_name,justification,result_note,actor) VALUES($1::uuid,'in_review',$2,$3,$4,$5)`, findingID, owner, justification, result, actor); err != nil {
				return nil, err
			}
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	items, err := s.ListActionMeasurements(ctx, findingID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].ID == id {
			return &items[i], nil
		}
	}
	return nil, ErrActionNotFound
}
