package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/osmendes/db-auditor/internal/config"
	"github.com/osmendes/db-auditor/internal/digest"
)

const digestCalculationVersion = "1"

// QueryDigestConfig is per-environment digest settings.
type QueryDigestConfig struct {
	EnvironmentID   string          `json:"environment_id"`
	WindowDays      int             `json:"window_days"`
	Thresholds      json.RawMessage `json:"thresholds"`
	DeliveryEnabled bool            `json:"delivery_enabled"`
	UpdatedAt       time.Time       `json:"updated_at"`
	UpdatedBy       string          `json:"updated_by"`
}

// QueryDigest is a persisted digest header.
type QueryDigest struct {
	ID                 string                `json:"id"`
	EnvironmentID      string                `json:"environment_id"`
	State              string                `json:"state"`
	Reason             string                `json:"reason"`
	WindowDays         int                   `json:"window_days"`
	PriorFrom          *time.Time            `json:"prior_from,omitempty"`
	PriorTo            *time.Time            `json:"prior_to,omitempty"`
	CurrentFrom        *time.Time            `json:"current_from,omitempty"`
	CurrentTo          *time.Time            `json:"current_to,omitempty"`
	PriorFirstRunID    *string               `json:"prior_first_run_id,omitempty"`
	PriorLastRunID     *string               `json:"prior_last_run_id,omitempty"`
	CurrentFirstRunID  *string               `json:"current_first_run_id,omitempty"`
	CurrentLastRunID   *string               `json:"current_last_run_id,omitempty"`
	WorkloadLimitN     int                   `json:"workload_limit_n"`
	SubwindowNote      string                `json:"subwindow_note"`
	CoverageNotes      []string              `json:"coverage_notes"`
	CalculationVersion string                `json:"calculation_version"`
	Markdown           string                `json:"markdown,omitempty"`
	RegressionCount    int                   `json:"regression_count"`
	NewCount           int                   `json:"new_count"`
	GoneCount          int                   `json:"gone_count"`
	ImprovementCount   int                   `json:"improvement_count"`
	CreatedAt          time.Time             `json:"created_at"`
	Items              []QueryDigestItem     `json:"items,omitempty"`
	Deliveries         []QueryDigestDelivery `json:"deliveries,omitempty"`
}

// QueryDigestItem is one classified fingerprint row (no SQL text).
type QueryDigestItem struct {
	ID                string   `json:"id"`
	DigestID          string   `json:"digest_id"`
	Classification    string   `json:"classification"`
	Severity          string   `json:"severity"`
	Fingerprint       string   `json:"fingerprint"`
	QueryID           string   `json:"query_id"`
	DatabaseName      string   `json:"database_name"`
	ReferencedObjects []string `json:"referenced_objects"`
	PriorCalls        int64    `json:"prior_calls"`
	CurrentCalls      int64    `json:"current_calls"`
	PriorMeanMS       float64  `json:"prior_mean_ms"`
	CurrentMeanMS     float64  `json:"current_mean_ms"`
	PriorTotalMS      float64  `json:"prior_total_ms"`
	CurrentTotalMS    float64  `json:"current_total_ms"`
	MeanChangePct     float64  `json:"mean_change_pct"`
	TotalChangePct    float64  `json:"total_change_pct"`
	PriorShare        float64  `json:"prior_share"`
	CurrentShare      float64  `json:"current_share"`
	FindingID         *string  `json:"finding_id,omitempty"`
	RegressionAlertID *string  `json:"regression_alert_id,omitempty"`
	RelatedFindingIDs []string `json:"related_finding_ids"`
	SortRank          int      `json:"sort_rank"`
}

// QueryDigestDelivery records one delivery attempt.
type QueryDigestDelivery struct {
	ID             string    `json:"id"`
	DigestID       string    `json:"digest_id"`
	DestinationID  *string   `json:"destination_id,omitempty"`
	Channel        string    `json:"channel"`
	TargetMasked   string    `json:"target_masked"`
	Status         string    `json:"status"`
	Attempt        int       `json:"attempt"`
	ErrorSanitized string    `json:"error_sanitized"`
	AttemptedAt    time.Time `json:"attempted_at"`
}

func workloadLimitN() int {
	if raw := strings.TrimSpace(os.Getenv("AUDITOR_WORKLOAD_LIMIT")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			return n
		}
	}
	return 500
}

// GetQueryDigestConfig returns config or defaults when none is stored.
func (s *Store) GetQueryDigestConfig(ctx context.Context, env string) (*QueryDigestConfig, error) {
	var c QueryDigestConfig
	err := s.pool.QueryRow(ctx, `SELECT environment_id::text,window_days,thresholds,delivery_enabled,updated_at,updated_by
FROM query_digest_config WHERE environment_id=$1::uuid`, env).Scan(
		&c.EnvironmentID, &c.WindowDays, &c.Thresholds, &c.DeliveryEnabled, &c.UpdatedAt, &c.UpdatedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return &QueryDigestConfig{
			EnvironmentID:   env,
			WindowDays:      7,
			Thresholds:      json.RawMessage(`{}`),
			DeliveryEnabled: true,
		}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(c.Thresholds) == 0 {
		c.Thresholds = json.RawMessage(`{}`)
	}
	return &c, nil
}

// UpsertQueryDigestConfig stores operator settings.
func (s *Store) UpsertQueryDigestConfig(ctx context.Context, env string, windowDays int, thresholds json.RawMessage, delivery bool, actor string) (*QueryDigestConfig, error) {
	if windowDays < 1 || windowDays > 90 {
		return nil, fmt.Errorf("window_days out of range")
	}
	if len(thresholds) == 0 {
		thresholds = json.RawMessage(`{}`)
	}
	if !json.Valid(thresholds) {
		return nil, fmt.Errorf("thresholds invalid json")
	}
	var c QueryDigestConfig
	err := s.pool.QueryRow(ctx, `INSERT INTO query_digest_config(environment_id,window_days,thresholds,delivery_enabled,updated_at,updated_by)
VALUES($1::uuid,$2,$3::jsonb,$4,now(),$5)
ON CONFLICT(environment_id) DO UPDATE SET window_days=EXCLUDED.window_days,thresholds=EXCLUDED.thresholds,
delivery_enabled=EXCLUDED.delivery_enabled,updated_at=now(),updated_by=EXCLUDED.updated_by
RETURNING environment_id::text,window_days,thresholds,delivery_enabled,updated_at,updated_by`,
		env, windowDays, thresholds, delivery, actor).Scan(
		&c.EnvironmentID, &c.WindowDays, &c.Thresholds, &c.DeliveryEnabled, &c.UpdatedAt, &c.UpdatedBy)
	return &c, err
}

func thresholdsFromJSON(raw json.RawMessage) digest.Thresholds {
	th := digest.DefaultThresholds()
	if len(raw) == 0 || string(raw) == "{}" || string(raw) == "null" {
		return th
	}
	var overlay struct {
		RegressionMeanPct  *float64 `json:"regression_mean_pct"`
		RegressionAbsMS    *float64 `json:"regression_abs_ms"`
		MinCalls           *int64   `json:"min_calls"`
		HighTotalPct       *float64 `json:"high_total_pct"`
		HighShareOfDB      *float64 `json:"high_share_of_db"`
		NewShareOfCurrent  *float64 `json:"new_share_of_current"`
		GoneShareOfPrior   *float64 `json:"gone_share_of_prior"`
		ImprovementMeanPct *float64 `json:"improvement_mean_pct"`
	}
	if json.Unmarshal(raw, &overlay) != nil {
		return th
	}
	if overlay.RegressionMeanPct != nil {
		th.RegressionMeanPct = *overlay.RegressionMeanPct
	}
	if overlay.RegressionAbsMS != nil {
		th.RegressionAbsMS = *overlay.RegressionAbsMS
	}
	if overlay.MinCalls != nil {
		th.MinCalls = *overlay.MinCalls
	}
	if overlay.HighTotalPct != nil {
		th.HighTotalPct = *overlay.HighTotalPct
	}
	if overlay.HighShareOfDB != nil {
		th.HighShareOfDB = *overlay.HighShareOfDB
	}
	if overlay.NewShareOfCurrent != nil {
		th.NewShareOfCurrent = *overlay.NewShareOfCurrent
	}
	if overlay.GoneShareOfPrior != nil {
		th.GoneShareOfPrior = *overlay.GoneShareOfPrior
	}
	if overlay.ImprovementMeanPct != nil {
		th.ImprovementMeanPct = *overlay.ImprovementMeanPct
	}
	return th
}

// ListQueryDigests returns paginated digest headers for an environment.
func (s *Store) ListQueryDigests(ctx context.Context, env string, limit, offset int) ([]QueryDigest, int, error) {
	if limit < 1 || limit > 500 || offset < 0 {
		return nil, 0, fmt.Errorf("invalid pagination")
	}
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM query_digest WHERE environment_id=$1::uuid`, env).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id::text,environment_id::text,state,reason,window_days,prior_from,prior_to,current_from,current_to,
prior_first_run_id::text,prior_last_run_id::text,current_first_run_id::text,current_last_run_id::text,
workload_limit_n,subwindow_note,coverage_notes,calculation_version,regression_count,new_count,gone_count,improvement_count,created_at
FROM query_digest WHERE environment_id=$1::uuid ORDER BY created_at DESC,id DESC LIMIT $2 OFFSET $3`, env, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []QueryDigest{}
	for rows.Next() {
		d, err := scanDigestHeader(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, d)
	}
	return items, total, rows.Err()
}

func scanDigestHeader(row interface{ Scan(dest ...any) error }) (QueryDigest, error) {
	var d QueryDigest
	var priorFirst, priorLast, currFirst, currLast *string
	if err := row.Scan(&d.ID, &d.EnvironmentID, &d.State, &d.Reason, &d.WindowDays,
		&d.PriorFrom, &d.PriorTo, &d.CurrentFrom, &d.CurrentTo,
		&priorFirst, &priorLast, &currFirst, &currLast,
		&d.WorkloadLimitN, &d.SubwindowNote, &d.CoverageNotes, &d.CalculationVersion,
		&d.RegressionCount, &d.NewCount, &d.GoneCount, &d.ImprovementCount, &d.CreatedAt); err != nil {
		return d, err
	}
	d.PriorFirstRunID, d.PriorLastRunID = priorFirst, priorLast
	d.CurrentFirstRunID, d.CurrentLastRunID = currFirst, currLast
	if d.CoverageNotes == nil {
		d.CoverageNotes = []string{}
	}
	return d, nil
}

// GetQueryDigest returns a digest with items and deliveries.
func (s *Store) GetQueryDigest(ctx context.Context, env, id string) (*QueryDigest, error) {
	row := s.pool.QueryRow(ctx, `SELECT id::text,environment_id::text,state,reason,window_days,prior_from,prior_to,current_from,current_to,
prior_first_run_id::text,prior_last_run_id::text,current_first_run_id::text,current_last_run_id::text,
workload_limit_n,subwindow_note,coverage_notes,calculation_version,markdown,regression_count,new_count,gone_count,improvement_count,created_at
FROM query_digest WHERE environment_id=$1::uuid AND id=$2::uuid`, env, id)
	var d QueryDigest
	var priorFirst, priorLast, currFirst, currLast *string
	err := row.Scan(&d.ID, &d.EnvironmentID, &d.State, &d.Reason, &d.WindowDays,
		&d.PriorFrom, &d.PriorTo, &d.CurrentFrom, &d.CurrentTo,
		&priorFirst, &priorLast, &currFirst, &currLast,
		&d.WorkloadLimitN, &d.SubwindowNote, &d.CoverageNotes, &d.CalculationVersion, &d.Markdown,
		&d.RegressionCount, &d.NewCount, &d.GoneCount, &d.ImprovementCount, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	d.PriorFirstRunID, d.PriorLastRunID = priorFirst, priorLast
	d.CurrentFirstRunID, d.CurrentLastRunID = currFirst, currLast
	if d.CoverageNotes == nil {
		d.CoverageNotes = []string{}
	}
	items, err := s.listDigestItems(ctx, d.ID)
	if err != nil {
		return nil, err
	}
	d.Items = items
	deliveries, err := s.listDigestDeliveries(ctx, d.ID)
	if err != nil {
		return nil, err
	}
	d.Deliveries = deliveries
	return &d, nil
}

func (s *Store) listDigestItems(ctx context.Context, digestID string) ([]QueryDigestItem, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text,digest_id::text,classification,severity,fingerprint,query_id,database_name,referenced_objects,
prior_calls,current_calls,prior_mean_ms,current_mean_ms,prior_total_ms,current_total_ms,mean_change_pct,total_change_pct,
prior_share,current_share,finding_id::text,regression_alert_id::text,related_finding_ids,sort_rank
FROM query_digest_item WHERE digest_id=$1::uuid ORDER BY classification,sort_rank,id`, digestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []QueryDigestItem{}
	for rows.Next() {
		var it QueryDigestItem
		var findingID, alertID *string
		var related []string
		if err := rows.Scan(&it.ID, &it.DigestID, &it.Classification, &it.Severity, &it.Fingerprint, &it.QueryID, &it.DatabaseName, &it.ReferencedObjects,
			&it.PriorCalls, &it.CurrentCalls, &it.PriorMeanMS, &it.CurrentMeanMS, &it.PriorTotalMS, &it.CurrentTotalMS, &it.MeanChangePct, &it.TotalChangePct,
			&it.PriorShare, &it.CurrentShare, &findingID, &alertID, &related, &it.SortRank); err != nil {
			return nil, err
		}
		it.FindingID, it.RegressionAlertID = findingID, alertID
		if it.ReferencedObjects == nil {
			it.ReferencedObjects = []string{}
		}
		it.RelatedFindingIDs = related
		if it.RelatedFindingIDs == nil {
			it.RelatedFindingIDs = []string{}
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *Store) listDigestDeliveries(ctx context.Context, digestID string) ([]QueryDigestDelivery, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text,digest_id::text,destination_id::text,channel,target_masked,status,attempt,error_sanitized,attempted_at
FROM query_digest_delivery WHERE digest_id=$1::uuid ORDER BY attempted_at DESC,id DESC`, digestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []QueryDigestDelivery{}
	for rows.Next() {
		var d QueryDigestDelivery
		var dest *string
		if err := rows.Scan(&d.ID, &d.DigestID, &dest, &d.Channel, &d.TargetMasked, &d.Status, &d.Attempt, &d.ErrorSanitized, &d.AttemptedAt); err != nil {
			return nil, err
		}
		d.DestinationID = dest
		out = append(out, d)
	}
	return out, rows.Err()
}

// collectionEdge is an internal row for window edge selection.
type collectionEdge struct {
	ID             string
	StartedAt      time.Time
	Status         string
	Completeness   string
	AnalysisStatus string
	StatsReset     *time.Time
	WorkloadReset  *time.Time
}

// listCollectionEdges returns successful/partial runs in [from,to] with completeness.
func (s *Store) listCollectionEdges(ctx context.Context, env string, from, to time.Time) ([]digest.CollectionPoint, error) {
	rows, err := s.pool.Query(ctx, `SELECT r.id::text,r.started_at,r.status,
COALESCE(a.status,''),
(SELECT max(stats_reset) FROM table_snapshot t WHERE t.audit_run_id=r.id),
(SELECT max(stats_reset) FROM workload_snapshot w WHERE w.audit_run_id=r.id)
FROM audit_run r
LEFT JOIN analysis_run a ON a.audit_run_id=r.id
WHERE r.environment_id=$1::uuid AND r.started_at>=$2 AND r.started_at<=$3
AND r.status IN ('success','partial_success')
ORDER BY r.started_at ASC,r.id ASC`, env, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []digest.CollectionPoint
	for rows.Next() {
		var e collectionEdge
		if err := rows.Scan(&e.ID, &e.StartedAt, &e.Status, &e.AnalysisStatus, &e.StatsReset, &e.WorkloadReset); err != nil {
			return nil, err
		}
		cov, err := s.GetSnapshotCompleteness(ctx, env, e.ID)
		if err != nil {
			return nil, err
		}
		completeness := "unknown"
		if cov != nil {
			completeness = cov.Completeness
		}
		out = append(out, digest.CollectionPoint{
			AuditRunID: e.ID, StartedAt: e.StartedAt, Completeness: completeness,
			RunStatus: e.Status, AnalysisStatus: e.AnalysisStatus,
			StatsReset: e.StatsReset, WorkloadReset: e.WorkloadReset,
		})
	}
	return out, rows.Err()
}

// listWorkloadForRuns pages workload_snapshot for the given run IDs.
func (s *Store) listWorkloadForRuns(ctx context.Context, env string, runIDs []string) ([]digest.WorkloadPoint, error) {
	if len(runIDs) == 0 {
		return nil, nil
	}
	const page = 2000
	var out []digest.WorkloadPoint
	offset := 0
	for {
		rows, err := s.pool.Query(ctx, `SELECT audit_run_id::text,database_name,query_fingerprint,query_id,calls,total_exec_time_ms,mean_exec_time_ms,
rows_total,shared_blocks_read,referenced_objects,stats_reset,collected_at
FROM workload_snapshot WHERE environment_id=$1::uuid AND audit_run_id=ANY($2::uuid[])
ORDER BY audit_run_id,database_name,query_fingerprint
LIMIT $3 OFFSET $4`, env, runIDs, page, offset)
		if err != nil {
			return nil, err
		}
		n := 0
		for rows.Next() {
			var p digest.WorkloadPoint
			if err := rows.Scan(&p.AuditRunID, &p.DatabaseName, &p.Fingerprint, &p.QueryID, &p.Calls, &p.TotalExecTimeMS, &p.MeanExecTimeMS,
				&p.RowsTotal, &p.SharedBlocksRead, &p.ReferencedObjects, &p.StatsReset, &p.CollectedAt); err != nil {
				rows.Close()
				return nil, err
			}
			if p.ReferencedObjects == nil {
				p.ReferencedObjects = []string{}
			}
			out = append(out, p)
			n++
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		if n < page {
			break
		}
		offset += page
	}
	return out, nil
}

func runIDsFromPoints(points []digest.CollectionPoint) []string {
	ids := make([]string, 0, len(points))
	for _, p := range points {
		ids = append(ids, p.AuditRunID)
	}
	return ids
}

// GenerateQueryDigest builds and persists a digest for the environment.
// reference is the end of the current window (default now).
func (s *Store) GenerateQueryDigest(ctx context.Context, env string, reference time.Time) (*QueryDigest, error) {
	if reference.IsZero() {
		reference = time.Now().UTC()
	}
	cfg, err := s.GetQueryDigestConfig(ctx, env)
	if err != nil {
		return nil, err
	}
	engine, err := s.GetEnvironmentEngine(ctx, env)
	if err != nil {
		return nil, err
	}
	envName := env
	_ = s.pool.QueryRow(ctx, `SELECT name FROM audit_environment WHERE id=$1::uuid`, env).Scan(&envName)

	if engine != "postgresql" && engine != "timescaledb" {
		return s.persistDigest(ctx, env, envName, cfg, digest.WindowResult{}, digest.WindowResult{}, digest.DigestSections{},
			"not_applicable", "não aplicável a este mecanismo", "", nil)
	}

	// Capability: pg_stat_statements absent → not_applicable.
	var hasWorkload bool
	_ = s.pool.QueryRow(ctx, `SELECT EXISTS(
SELECT 1 FROM workload_snapshot WHERE environment_id=$1::uuid
UNION ALL SELECT 1 FROM audit_run_coverage WHERE audit_run_id IN (
 SELECT id FROM audit_run WHERE environment_id=$1::uuid ORDER BY started_at DESC LIMIT 5)
 AND collector_name LIKE '%workload%' AND status='success' LIMIT 1)`, env).Scan(&hasWorkload)
	var anySnapshot bool
	_ = s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM workload_snapshot WHERE environment_id=$1::uuid LIMIT 1)`, env).Scan(&anySnapshot)
	if !anySnapshot {
		// Check latest capability / coverage hint for missing extension.
		var missingExt bool
		_ = s.pool.QueryRow(ctx, `SELECT EXISTS(
SELECT 1 FROM audit_run_coverage c JOIN audit_run r ON r.id=c.audit_run_id
WHERE r.environment_id=$1::uuid AND c.collector_name LIKE '%workload%'
AND (c.status='failed' OR c.warning ILIKE '%pg_stat_statements%' OR c.error ILIKE '%pg_stat_statements%')
ORDER BY r.started_at DESC LIMIT 5)`, env).Scan(&missingExt)
		if missingExt || !hasWorkload {
			return s.persistDigest(ctx, env, envName, cfg, digest.WindowResult{}, digest.WindowResult{}, digest.DigestSections{},
				"not_applicable", "não aplicável — extensão pg_stat_statements ausente ou sem evidência de workload", "", nil)
		}
	}

	days := time.Duration(cfg.WindowDays) * 24 * time.Hour
	currentFrom := reference.Add(-days)
	priorFrom := currentFrom.Add(-days)
	priorTo := currentFrom

	priorPoints, err := s.listCollectionEdges(ctx, env, priorFrom, priorTo)
	if err != nil {
		return nil, err
	}
	currentPoints, err := s.listCollectionEdges(ctx, env, currentFrom, reference)
	if err != nil {
		return nil, err
	}
	priorWorkload, err := s.listWorkloadForRuns(ctx, env, runIDsFromPoints(priorPoints))
	if err != nil {
		return nil, err
	}
	currentWorkload, err := s.listWorkloadForRuns(ctx, env, runIDsFromPoints(currentPoints))
	if err != nil {
		return nil, err
	}
	maxGap := RegressionMaxGap()
	// Allow digest windows up to 2× configured days plus a day of slack for sparse cadences.
	windowGap := days*2 + 24*time.Hour
	if windowGap < maxGap {
		windowGap = maxGap
	}
	priorWin := digest.ResolveWindow(priorPoints, priorWorkload, windowGap)
	currWin := digest.ResolveWindow(currentPoints, currentWorkload, windowGap)
	if !priorWin.OK || !currWin.OK {
		reason := "sem janela comparável"
		if !priorWin.OK && priorWin.Reason != "" {
			reason = "janela anterior: " + priorWin.Reason
		} else if !currWin.OK && currWin.Reason != "" {
			reason = "janela atual: " + currWin.Reason
		}
		return s.persistDigest(ctx, env, envName, cfg, priorWin, currWin, digest.DigestSections{},
			"no_comparable_window", reason, "", nil)
	}

	th := thresholdsFromJSON(cfg.Thresholds)
	classified := digest.CompareWindows(priorWin, currWin, th)
	sections := digest.SelectTops(classified, th)

	related, err := s.attachDigestLinks(ctx, env, &sections)
	if err != nil {
		return nil, err
	}
	subNote := currWin.SubwindowNote
	if priorWin.UsedSubwindow && priorWin.SubwindowNote != "" {
		if subNote != "" {
			subNote += "; "
		}
		subNote += "anterior: " + priorWin.SubwindowNote
	}
	notes := []string{
		fmt.Sprintf("Top N do coletor: %d (AUDITOR_WORKLOAD_LIMIT).", workloadLimitN()),
	}
	if subNote != "" {
		notes = append(notes, subNote)
	}
	return s.persistDigest(ctx, env, envName, cfg, priorWin, currWin, sections, "ok", "", subNote, related)
}

type digestLinkBag struct {
	findingByFP map[string]string
	alertByFP   map[string]string
	relatedByFP map[string][]string
}

func (s *Store) attachDigestLinks(ctx context.Context, env string, sections *digest.DigestSections) (*digestLinkBag, error) {
	bag := &digestLinkBag{
		findingByFP: map[string]string{},
		alertByFP:   map[string]string{},
		relatedByFP: map[string][]string{},
	}
	all := append(append(append(append([]digest.ClassifiedItem{}, sections.Regressions...), sections.NewQueries...), sections.GoneQueries...), sections.Improvements...)
	objects := map[string]struct{}{}
	fps := map[string]struct{}{}
	for _, it := range all {
		fps[it.Fingerprint] = struct{}{}
		for _, o := range it.ReferencedObjects {
			objects[o] = struct{}{}
		}
	}
	// Open workload regression alerts — link only, never create.
	rows, err := s.pool.Query(ctx, `SELECT id::text,evidence FROM regression_alert
WHERE environment_id=$1::uuid AND category='workload' AND status='open' ORDER BY created_at DESC LIMIT 200`, env)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id string
		var evidence json.RawMessage
		if err := rows.Scan(&id, &evidence); err != nil {
			rows.Close()
			return nil, err
		}
		var payload struct {
			Keys []string `json:"keys"`
		}
		_ = json.Unmarshal(evidence, &payload)
		for _, k := range payload.Keys {
			if _, ok := fps[k]; ok {
				if _, exists := bag.alertByFP[k]; !exists {
					bag.alertByFP[k] = id
				}
			}
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Findings: index.candidate / performance.workload_scan / slow_query by object or fingerprint in object_key/dedup.
	frows, err := s.pool.Query(ctx, `SELECT id::text,finding_type,object_key,dedup_key,schema_name,object_name
FROM finding WHERE environment_id=$1::uuid AND status='open'
AND finding_type IN ('index.candidate','performance.workload_scan','performance.slow_query','performance.workload_heavy')
ORDER BY last_seen_at DESC LIMIT 500`, env)
	if err != nil {
		return nil, err
	}
	defer frows.Close()
	for frows.Next() {
		var id, ftype, okey, dkey, schema, oname string
		if err := frows.Scan(&id, &ftype, &okey, &dkey, &schema, &oname); err != nil {
			return nil, err
		}
		qual := schema + "." + oname
		for fp := range fps {
			if strings.Contains(okey, fp) || strings.Contains(dkey, fp) {
				if ftype == "performance.slow_query" || strings.HasPrefix(ftype, "performance.workload") {
					if _, ok := bag.findingByFP[fp]; !ok {
						bag.findingByFP[fp] = id
					}
				}
				bag.relatedByFP[fp] = appendUnique(bag.relatedByFP[fp], id)
			}
		}
		for _, it := range all {
			for _, obj := range it.ReferencedObjects {
				if obj == qual || obj == oname || strings.HasSuffix(obj, "."+oname) {
					if ftype == "index.candidate" || ftype == "performance.workload_scan" {
						bag.relatedByFP[it.Fingerprint] = appendUnique(bag.relatedByFP[it.Fingerprint], id)
					}
				}
			}
			_ = objects
		}
	}
	return bag, frows.Err()
}

func appendUnique(xs []string, v string) []string {
	for _, x := range xs {
		if x == v {
			return xs
		}
	}
	return append(xs, v)
}

func (s *Store) persistDigest(ctx context.Context, env, envName string, cfg *QueryDigestConfig,
	prior, current digest.WindowResult, sections digest.DigestSections,
	state, reason, subNote string, links *digestLinkBag) (*QueryDigest, error) {

	mdIn := digest.MarkdownInput{
		EnvironmentName: envName,
		EnvironmentID:   env,
		State:           state,
		Reason:          reason,
		WorkloadLimitN:  workloadLimitN(),
		SubwindowNote:   subNote,
		Sections:        sections,
		CoverageNotes: []string{
			fmt.Sprintf("Top N do coletor: %d.", workloadLimitN()),
		},
	}
	if prior.OK {
		mdIn.PriorFrom, mdIn.PriorTo = prior.First.StartedAt, prior.Last.StartedAt
		mdIn.PriorFirstRun, mdIn.PriorLastRun = prior.First.AuditRunID, prior.Last.AuditRunID
	}
	if current.OK {
		mdIn.CurrentFrom, mdIn.CurrentTo = current.First.StartedAt, current.Last.StartedAt
		mdIn.CurrentFirstRun, mdIn.CurrentLastRun = current.First.AuditRunID, current.Last.AuditRunID
	}
	markdown := digest.RenderMarkdown(mdIn)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var d QueryDigest
	err = tx.QueryRow(ctx, `INSERT INTO query_digest(
environment_id,state,reason,window_days,prior_from,prior_to,current_from,current_to,
prior_first_run_id,prior_last_run_id,current_first_run_id,current_last_run_id,
workload_limit_n,subwindow_note,coverage_notes,calculation_version,markdown,
regression_count,new_count,gone_count,improvement_count)
VALUES($1::uuid,$2,$3,$4,$5,$6,$7,$8,
NULLIF($9,'')::uuid,NULLIF($10,'')::uuid,NULLIF($11,'')::uuid,NULLIF($12,'')::uuid,
$13,$14,$15,$16,$17,$18,$19,$20,$21)
RETURNING id::text,environment_id::text,state,reason,window_days,prior_from,prior_to,current_from,current_to,
prior_first_run_id::text,prior_last_run_id::text,current_first_run_id::text,current_last_run_id::text,
workload_limit_n,subwindow_note,coverage_notes,calculation_version,markdown,regression_count,new_count,gone_count,improvement_count,created_at`,
		env, state, reason, cfg.WindowDays,
		nullTime(prior.OK, prior.First.StartedAt), nullTime(prior.OK, prior.Last.StartedAt),
		nullTime(current.OK, current.First.StartedAt), nullTime(current.OK, current.Last.StartedAt),
		runOrEmpty(prior.OK, prior.First.AuditRunID), runOrEmpty(prior.OK, prior.Last.AuditRunID),
		runOrEmpty(current.OK, current.First.AuditRunID), runOrEmpty(current.OK, current.Last.AuditRunID),
		workloadLimitN(), subNote, mdIn.CoverageNotes, digestCalculationVersion, markdown,
		len(sections.Regressions), len(sections.NewQueries), len(sections.GoneQueries), len(sections.Improvements),
	).Scan(&d.ID, &d.EnvironmentID, &d.State, &d.Reason, &d.WindowDays,
		&d.PriorFrom, &d.PriorTo, &d.CurrentFrom, &d.CurrentTo,
		&d.PriorFirstRunID, &d.PriorLastRunID, &d.CurrentFirstRunID, &d.CurrentLastRunID,
		&d.WorkloadLimitN, &d.SubwindowNote, &d.CoverageNotes, &d.CalculationVersion, &d.Markdown,
		&d.RegressionCount, &d.NewCount, &d.GoneCount, &d.ImprovementCount, &d.CreatedAt)
	if err != nil {
		return nil, err
	}

	type ranked struct {
		it   digest.ClassifiedItem
		rank int
	}
	var rankedItems []ranked
	for i, it := range sections.Regressions {
		rankedItems = append(rankedItems, ranked{it, i})
	}
	for i, it := range sections.NewQueries {
		rankedItems = append(rankedItems, ranked{it, i})
	}
	for i, it := range sections.GoneQueries {
		rankedItems = append(rankedItems, ranked{it, i})
	}
	for i, it := range sections.Improvements {
		rankedItems = append(rankedItems, ranked{it, i})
	}
	d.Items = []QueryDigestItem{}
	for _, r := range rankedItems {
		it := r.it
		var findingID, alertID *string
		var related []string
		if links != nil {
			if id, ok := links.findingByFP[it.Fingerprint]; ok {
				findingID = &id
			}
			if id, ok := links.alertByFP[it.Fingerprint]; ok {
				alertID = &id
			}
			related = links.relatedByFP[it.Fingerprint]
		}
		if related == nil {
			related = []string{}
		}
		var itemID string
		err = tx.QueryRow(ctx, `INSERT INTO query_digest_item(
digest_id,classification,severity,fingerprint,query_id,database_name,referenced_objects,
prior_calls,current_calls,prior_mean_ms,current_mean_ms,prior_total_ms,current_total_ms,
mean_change_pct,total_change_pct,prior_share,current_share,finding_id,regression_alert_id,related_finding_ids,sort_rank)
VALUES($1::uuid,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,NULLIF($18,'')::uuid,NULLIF($19,'')::uuid,$20::uuid[],$21)
RETURNING id::text`,
			d.ID, it.Classification, it.Severity, it.Fingerprint, it.QueryID, it.DatabaseName, it.ReferencedObjects,
			it.PriorCalls, it.CurrentCalls, it.PriorMeanMS, it.CurrentMeanMS, it.PriorTotalMS, it.CurrentTotalMS,
			it.MeanChangePct, it.TotalChangePct, it.PriorShare, it.CurrentShare,
			deref(findingID), deref(alertID), related, r.rank,
		).Scan(&itemID)
		if err != nil {
			return nil, err
		}
		d.Items = append(d.Items, QueryDigestItem{
			ID: itemID, DigestID: d.ID, Classification: it.Classification, Severity: it.Severity,
			Fingerprint: it.Fingerprint, QueryID: it.QueryID, DatabaseName: it.DatabaseName,
			ReferencedObjects: it.ReferencedObjects, PriorCalls: it.PriorCalls, CurrentCalls: it.CurrentCalls,
			PriorMeanMS: it.PriorMeanMS, CurrentMeanMS: it.CurrentMeanMS, PriorTotalMS: it.PriorTotalMS, CurrentTotalMS: it.CurrentTotalMS,
			MeanChangePct: it.MeanChangePct, TotalChangePct: it.TotalChangePct, PriorShare: it.PriorShare, CurrentShare: it.CurrentShare,
			FindingID: findingID, RegressionAlertID: alertID, RelatedFindingIDs: related, SortRank: r.rank,
		})
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	d.Deliveries = []QueryDigestDelivery{}
	return &d, nil
}

func nullTime(ok bool, t time.Time) *time.Time {
	if !ok || t.IsZero() {
		return nil
	}
	return &t
}

func runOrEmpty(ok bool, id string) string {
	if !ok {
		return ""
	}
	return id
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// MaybeGenerateQueryDigestAfterRun generates a digest when the completed run
// closes a configured window (same hook point as EvaluateRegressionAlerts).
func (s *Store) MaybeGenerateQueryDigestAfterRun(ctx context.Context, env, run string) error {
	cfg, err := s.GetQueryDigestConfig(ctx, env)
	if err != nil {
		return err
	}
	var started time.Time
	var status string
	err = s.pool.QueryRow(ctx, `SELECT started_at,status FROM audit_run WHERE environment_id=$1::uuid AND id=$2::uuid`, env, run).Scan(&started, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if status != "success" {
		return nil
	}
	cov, err := s.GetSnapshotCompleteness(ctx, env, run)
	if err != nil || cov == nil || cov.Completeness != "complete" {
		return err
	}
	var analysis string
	_ = s.pool.QueryRow(ctx, `SELECT COALESCE(status,'') FROM analysis_run WHERE audit_run_id=$1::uuid`, run).Scan(&analysis)
	if analysis != "success" {
		return nil
	}
	// Close the window when this run is the newest complete run at/after the
	// end of a window_days period since the previous digest (or since epoch).
	var lastDigest *time.Time
	_ = s.pool.QueryRow(ctx, `SELECT max(created_at) FROM query_digest WHERE environment_id=$1::uuid AND state IN ('ok','no_comparable_window','not_applicable')`, env).Scan(&lastDigest)
	days := time.Duration(cfg.WindowDays) * 24 * time.Hour
	if lastDigest != nil && started.Sub(*lastDigest) < days {
		return nil
	}
	d, err := s.GenerateQueryDigest(ctx, env, started)
	if err != nil {
		return err
	}
	if cfg.DeliveryEnabled && d != nil {
		if err := s.DeliverQueryDigest(ctx, d); err != nil {
			return fmt.Errorf("digest delivery: %s", config.SanitizeError(err))
		}
	}
	return nil
}
