package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/osmendes/db-auditor/internal/analyzer"
	"github.com/osmendes/db-auditor/internal/comparison"
	"github.com/osmendes/db-auditor/internal/observability"
	"github.com/osmendes/db-auditor/internal/schemadiff"
)

// SchemaChangeEventRow is one persisted timeline event.
type SchemaChangeEventRow struct {
	ID                 string    `json:"id"`
	EnvironmentID      string    `json:"environment_id"`
	PriorAuditRunID    string    `json:"prior_audit_run_id,omitempty"`
	CurrentAuditRunID  string    `json:"current_audit_run_id"`
	DetectedAt         time.Time `json:"detected_at"`
	ObjectType         string    `json:"object_type"`
	ObjectKey          string    `json:"object_key"`
	ChangeKind         string    `json:"change_kind"`
	PriorFingerprint   string    `json:"prior_fingerprint,omitempty"`
	CurrentFingerprint string    `json:"current_fingerprint,omitempty"`
	ChangedFields      []string  `json:"changed_fields,omitempty"`
	Correlation        string    `json:"correlation"`
	AnnotationID       string    `json:"annotation_id,omitempty"`
	RegistryVersion    string    `json:"registry_version,omitempty"`
	Confidence         string    `json:"confidence"`
}

// SchemaTimelineGapRow is a non-comparable interval.
type SchemaTimelineGapRow struct {
	ID                string    `json:"id"`
	EnvironmentID     string    `json:"environment_id"`
	PriorAuditRunID   string    `json:"prior_audit_run_id,omitempty"`
	CurrentAuditRunID string    `json:"current_audit_run_id,omitempty"`
	GapFrom           time.Time `json:"gap_from"`
	GapTo             time.Time `json:"gap_to"`
	Reason            string    `json:"reason"`
}

// SchemaTimelinePage is the API payload for timeline reads.
type SchemaTimelinePage struct {
	EnvironmentID string                 `json:"environment_id"`
	From          time.Time              `json:"from"`
	To            time.Time              `json:"to"`
	Events        []SchemaChangeEventRow `json:"events"`
	Gaps          []SchemaTimelineGapRow `json:"gaps"`
	Total         int                    `json:"total"`
	Limit         int                    `json:"limit"`
	Offset        int                    `json:"offset"`
}

const deploymentCorrelationTolerance = 30 * time.Minute

// RecordSchemaTimelineAfterRun compares the completed run with the previous
// complete run of the same profile/collector version and persists events or a gap.
// Findings for out-of-pipeline changes are upserted here (analysis already finished).
func (s *Store) RecordSchemaTimelineAfterRun(ctx context.Context, env, run string) error {
	var status, profile, collector string
	var started, finished time.Time
	err := s.pool.QueryRow(ctx, `
SELECT status, profile, collector_version, started_at, COALESCE(finished_at, started_at)
FROM audit_run WHERE id=$1::uuid AND environment_id=$2::uuid`, run, env).
		Scan(&status, &profile, &collector, &started, &finished)
	if err != nil {
		return err
	}
	if status != "success" {
		if status == "partial_success" {
			return s.recordTimelineGapForPartial(ctx, env, run, started, finished)
		}
		return nil
	}
	cov, err := s.GetSnapshotCompleteness(ctx, env, run)
	if err != nil {
		return err
	}
	if cov == nil || cov.Completeness != "complete" {
		return s.recordTimelineGapForPartial(ctx, env, run, started, finished)
	}

	var priorID string
	var priorStarted, priorFinished time.Time
	err = s.pool.QueryRow(ctx, `
SELECT id::text, started_at, COALESCE(finished_at, started_at)
FROM audit_run
WHERE environment_id=$1::uuid AND status='success' AND id<>$2::uuid
  AND profile=$3 AND collector_version=$4 AND started_at<$5
ORDER BY started_at DESC LIMIT 1`, env, run, profile, collector, started).
		Scan(&priorID, &priorStarted, &priorFinished)
	if err != nil {
		// First complete run — nothing to compare.
		return nil
	}
	priorCov, err := s.GetSnapshotCompleteness(ctx, env, priorID)
	if err != nil {
		return err
	}
	if priorCov == nil || priorCov.Completeness != "complete" {
		_, _ = s.pool.Exec(ctx, `
INSERT INTO schema_timeline_gap(environment_id, prior_audit_run_id, current_audit_run_id, gap_from, gap_to, reason)
VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5,'prior_incomplete')`, env, priorID, run, priorFinished, started)
		return nil
	}

	var incompleteBetween int
	_ = s.pool.QueryRow(ctx, `
SELECT count(*) FROM audit_run
WHERE environment_id=$1::uuid AND started_at>$2 AND started_at<$3
  AND status IN ('partial_success','failed','cancelled')`, env, priorStarted, started).Scan(&incompleteBetween)
	if incompleteBetween > 0 {
		_, _ = s.pool.Exec(ctx, `
INSERT INTO schema_timeline_gap(environment_id, prior_audit_run_id, current_audit_run_id, gap_from, gap_to, reason)
VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5,'partial_between')`, env, priorID, run, priorFinished, started)
		return nil
	}

	srcObjs, err := s.loadSchemaDiffObjects(ctx, env, priorID)
	if err != nil {
		return err
	}
	tgtObjs, err := s.loadSchemaDiffObjects(ctx, env, run)
	if err != nil {
		return err
	}
	diff := schemadiff.Diff(srcObjs, tgtObjs, nil)
	events := schemadiff.EventsFromDiff(diff)
	if len(events) == 0 {
		return nil
	}

	corrAnnotation, corrRegistry, registryVersion, accessState := s.correlatePipeline(ctx, env, priorStarted, finished, priorID, run)
	isProd, _ := s.EnvironmentIsProduction(ctx, env)

	var findings []analyzer.Finding
	for _, ev := range events {
		correlation := "none"
		annotationID := ""
		regVer := ""
		confidence := "high"
		switch {
		case corrAnnotation != "":
			correlation = "deployment"
			annotationID = corrAnnotation
		case corrRegistry:
			correlation = "migration_registry"
			regVer = registryVersion
			if accessState != "ok" {
				correlation = "not_verified"
				confidence = "low"
			} else if registryVersion != "" {
				// Tools without applied_at use version-changed correlation.
				confidence = "medium"
			}
		case accessState == "missing" || accessState == "denied" || accessState == "error":
			correlation = "not_verified"
			confidence = "low"
		}

		_, err = s.pool.Exec(ctx, `
INSERT INTO schema_change_event(
  environment_id, prior_audit_run_id, current_audit_run_id, detected_at,
  object_type, object_key, change_kind, prior_fingerprint, current_fingerprint,
  changed_fields, correlation, annotation_id, registry_version, confidence)
VALUES(
  $1::uuid, $2::uuid, $3::uuid, $4,
  $5, $6, $7, $8, $9,
  $10, $11, NULLIF($12,'')::uuid, $13, $14)`,
			env, priorID, run, finished,
			ev.ObjectType, ev.ObjectKey, ev.ChangeKind, ev.PriorFingerprint, ev.CurrentFingerprint,
			ev.ChangedFields, correlation, annotationID, regVer, confidence)
		if err != nil {
			return err
		}
		observability.DefaultMetrics.IncSchemaChangeEvent(ev.ChangeKind)

		if correlation == "none" {
			findings = append(findings, outOfPipelineFinding(env, run, ev, isProd))
		}
	}
	if len(findings) > 0 {
		if _, err := s.SaveAnalysisFindings(ctx, findings); err != nil {
			return err
		}
		s.NotifyDriftFindings(ctx, findings)
	}
	_ = s.emitEnvironmentDivergenceFindings(ctx, env, run, isProd)
	return nil
}

func (s *Store) recordTimelineGapForPartial(ctx context.Context, env, run string, started, finished time.Time) error {
	var priorID *string
	var priorFinished *time.Time
	var id string
	var fin time.Time
	err := s.pool.QueryRow(ctx, `
SELECT id::text, COALESCE(finished_at, started_at) FROM audit_run
WHERE environment_id=$1::uuid AND status='success' AND started_at<$2
ORDER BY started_at DESC LIMIT 1`, env, started).Scan(&id, &fin)
	if err == nil {
		priorID = &id
		priorFinished = &fin
	}
	gapFrom := started
	if priorFinished != nil {
		gapFrom = *priorFinished
	}
	_, err = s.pool.Exec(ctx, `
INSERT INTO schema_timeline_gap(environment_id, prior_audit_run_id, current_audit_run_id, gap_from, gap_to, reason)
VALUES($1::uuid, NULLIF($2,'')::uuid, $3::uuid, $4, $5, 'partial_run')`,
		env, strOrEmpty(priorID), run, gapFrom, finished)
	return err
}

func strOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func (s *Store) correlatePipeline(ctx context.Context, env string, from, to time.Time, priorRun, currentRun string) (annotationID string, registryOK bool, version, accessState string) {
	windowStart := from.Add(-deploymentCorrelationTolerance)
	windowEnd := to.Add(deploymentCorrelationTolerance)
	_ = s.pool.QueryRow(ctx, `
SELECT id::text FROM audit_annotation
WHERE environment_id=$1::uuid AND kind='deployment'
  AND created_at >= $2 AND created_at <= $3
ORDER BY created_at DESC LIMIT 1`, env, windowStart, windowEnd).Scan(&annotationID)

	accessState = "ok"
	var curVer, priorVer string
	var curAccess string
	err := s.pool.QueryRow(ctx, `
SELECT latest_version, access_state FROM migration_registry_snapshot
WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid
ORDER BY collected_at DESC LIMIT 1`, env, currentRun).Scan(&curVer, &curAccess)
	if err == nil {
		accessState = curAccess
		_ = s.pool.QueryRow(ctx, `
SELECT latest_version FROM migration_registry_snapshot
WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid
ORDER BY collected_at DESC LIMIT 1`, env, priorRun).Scan(&priorVer)
		if curAccess == "ok" && curVer != "" && curVer != priorVer {
			registryOK = true
			version = curVer
		}
	}
	return annotationID, registryOK, version, accessState
}

func outOfPipelineFinding(env, run string, ev schemadiff.ChangeEvent, isProd bool) analyzer.Finding {
	sev := schemadiff.OutOfPipelineSeverity(ev.ObjectType, ev.ChangeKind, false)
	if !isProd {
		sev = schemadiff.DowngradeSeverity(sev)
	}
	title := fmt.Sprintf("Mudança de schema fora do pipeline: %s %s", ev.ChangeKind, ev.ObjectKey)
	return analyzer.Finding{
		EnvironmentID:  env,
		AuditRunID:     run,
		FindingType:    "drift.out_of_pipeline",
		RuleID:         "drift.out_of_pipeline",
		RuleVersion:    "1",
		Category:       "structure",
		Confidence:     0.8,
		Severity:       analyzer.Severity(sev),
		Status:         analyzer.StatusOpen,
		Title:          title,
		Summary:        "Objeto alterado entre coletas completas sem anotação de implantação nem evidência no registro de migrations.",
		ObjectType:     ev.ObjectType,
		ObjectKey:      ev.ObjectKey,
		Recommendation: "Confirme se a mudança foi intencional; registre a implantação ou alinhe o registro de migrations.",
		Validation:     "Compare as duas execuções na linha do tempo de schema.",
		Evidence: map[string]any{
			"change_kind":         ev.ChangeKind,
			"prior_fingerprint":   ev.PriorFingerprint,
			"current_fingerprint": ev.CurrentFingerprint,
			"changed_fields":      ev.ChangedFields,
		},
		DedupKey: analyzer.DedupKey("drift.out_of_pipeline", ev.ObjectKey, ev.ChangeKind+"@"+run),
	}
}

func (s *Store) emitEnvironmentDivergenceFindings(ctx context.Context, env, run string, isProd bool) error {
	// Pairs come from accepted object_mapping where this env is source or target.
	rows, err := s.pool.Query(ctx, `
SELECT DISTINCT
  CASE WHEN source_environment_id=$1::uuid THEN target_environment_id::text ELSE source_environment_id::text END
FROM object_mapping
WHERE status='validated' AND (source_environment_id=$1::uuid OR target_environment_id=$1::uuid)`, env)
	if err != nil {
		return err
	}
	defer rows.Close()
	var peers []string
	for rows.Next() {
		var peer string
		if err := rows.Scan(&peer); err != nil {
			return err
		}
		peers = append(peers, peer)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	var findings []analyzer.Finding
	for _, peer := range peers {
		peerRun, _, err := s.latestCompleteRunID(ctx, peer)
		if err != nil || peerRun == "" {
			continue
		}
		// Require a previous complete pair as well (persistent across two collections).
		var priorPeer, priorSelf string
		_ = s.pool.QueryRow(ctx, `
SELECT id::text FROM audit_run WHERE environment_id=$1::uuid AND status='success' AND id<>$2::uuid
ORDER BY started_at DESC LIMIT 1`, peer, peerRun).Scan(&priorPeer)
		_ = s.pool.QueryRow(ctx, `
SELECT id::text FROM audit_run WHERE environment_id=$1::uuid AND status='success' AND id<>$2::uuid
ORDER BY started_at DESC LIMIT 1`, env, run).Scan(&priorSelf)
		if priorPeer == "" || priorSelf == "" {
			continue
		}
		curDiff, err := s.BuildSchemaDiff(ctx, SchemaDiffRequest{SourceEnv: env, SourceRun: run, TargetEnv: peer, TargetRun: peerRun, Limit: 5000})
		if err != nil || curDiff == nil {
			continue
		}
		priorDiff, err := s.BuildSchemaDiff(ctx, SchemaDiffRequest{SourceEnv: env, SourceRun: priorSelf, TargetEnv: peer, TargetRun: priorPeer, Limit: 5000})
		if err != nil || priorDiff == nil {
			continue
		}
		persistent := intersectDriftKeys(curDiff.Objects, priorDiff.Objects)
		for _, key := range persistent {
			sev := "medium"
			if !isProd {
				sev = schemadiff.DowngradeSeverity(sev)
			}
			findings = append(findings, analyzer.Finding{
				EnvironmentID:  env,
				AuditRunID:     run,
				FindingType:    "drift.environment_divergence",
				RuleID:         "drift.environment_divergence",
				RuleVersion:    "1",
				Category:       "structure",
				Confidence:     0.7,
				Severity:       analyzer.Severity(sev),
				Status:         analyzer.StatusOpen,
				Title:          "Divergência persistente entre ambientes: " + key,
				Summary:        "O objeto diverge do ambiente pareado em duas coletas completas consecutivas de cada lado.",
				ObjectKey:      key,
				Recommendation: "Alinhe os ambientes ou confirme que a diferença é intencional.",
				Validation:     "Compare as execuções na tela Desvio de schema.",
				Evidence:       map[string]any{"peer_environment_id": peer, "peer_run_id": peerRun},
				DedupKey:       analyzer.DedupKey("drift.environment_divergence", key, peer),
			})
		}
	}
	if len(findings) == 0 {
		return nil
	}
	if _, err := s.SaveAnalysisFindings(ctx, findings); err != nil {
		return err
	}
	s.NotifyDriftFindings(ctx, findings)
	return nil
}

func intersectDriftKeys(a, b []comparison.ObjectDiff) []string {
	set := map[string]struct{}{}
	for _, o := range a {
		if o.Status == comparison.StatusMatch {
			continue
		}
		set[o.ObjectType+"|"+o.ObjectKey] = struct{}{}
	}
	var out []string
	for _, o := range b {
		if o.Status == comparison.StatusMatch {
			continue
		}
		k := o.ObjectType + "|" + o.ObjectKey
		if _, ok := set[k]; ok {
			out = append(out, o.ObjectKey)
		}
	}
	return out
}

// ListSchemaTimeline returns persisted events (no recalculation) and gaps.
func (s *Store) ListSchemaTimeline(ctx context.Context, env string, from, to time.Time, limit, offset int) (*SchemaTimelinePage, error) {
	if to.IsZero() {
		to = time.Now().UTC()
	}
	if from.IsZero() {
		from = to.Add(-90 * 24 * time.Hour)
	}
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	var total int
	if err := s.pool.QueryRow(ctx, `
SELECT count(*) FROM schema_change_event
WHERE environment_id=$1::uuid AND detected_at>=$2 AND detected_at<=$3`, env, from, to).Scan(&total); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
SELECT id::text, environment_id::text, COALESCE(prior_audit_run_id::text,''), current_audit_run_id::text,
       detected_at, object_type, object_key, change_kind, prior_fingerprint, current_fingerprint,
       changed_fields, correlation, COALESCE(annotation_id::text,''), registry_version, confidence
FROM schema_change_event
WHERE environment_id=$1::uuid AND detected_at>=$2 AND detected_at<=$3
ORDER BY detected_at DESC, id DESC
LIMIT $4 OFFSET $5`, env, from, to, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []SchemaChangeEventRow{}
	for rows.Next() {
		var e SchemaChangeEventRow
		if err := rows.Scan(&e.ID, &e.EnvironmentID, &e.PriorAuditRunID, &e.CurrentAuditRunID,
			&e.DetectedAt, &e.ObjectType, &e.ObjectKey, &e.ChangeKind, &e.PriorFingerprint, &e.CurrentFingerprint,
			&e.ChangedFields, &e.Correlation, &e.AnnotationID, &e.RegistryVersion, &e.Confidence); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	gapRows, err := s.pool.Query(ctx, `
SELECT id::text, environment_id::text, COALESCE(prior_audit_run_id::text,''), COALESCE(current_audit_run_id::text,''),
       gap_from, gap_to, reason
FROM schema_timeline_gap
WHERE environment_id=$1::uuid AND gap_to>=$2 AND gap_from<=$3
ORDER BY gap_to DESC`, env, from, to)
	if err != nil {
		return nil, err
	}
	defer gapRows.Close()
	gaps := []SchemaTimelineGapRow{}
	for gapRows.Next() {
		var g SchemaTimelineGapRow
		if err := gapRows.Scan(&g.ID, &g.EnvironmentID, &g.PriorAuditRunID, &g.CurrentAuditRunID, &g.GapFrom, &g.GapTo, &g.Reason); err != nil {
			return nil, err
		}
		gaps = append(gaps, g)
	}
	return &SchemaTimelinePage{
		EnvironmentID: env, From: from, To: to,
		Events: events, Gaps: gaps, Total: total, Limit: limit, Offset: offset,
	}, gapRows.Err()
}

// EnvironmentIsProduction reads the production flag.
func (s *Store) EnvironmentIsProduction(ctx context.Context, env string) (bool, error) {
	var v bool
	err := s.pool.QueryRow(ctx, `SELECT COALESCE(is_production,false) FROM audit_environment WHERE id=$1::uuid`, env).Scan(&v)
	return v, err
}

// SetEnvironmentProduction updates is_production (operator).
func (s *Store) SetEnvironmentProduction(ctx context.Context, env string, isProd bool, actor string) error {
	tag, err := s.pool.Exec(ctx, `
UPDATE audit_environment SET is_production=$2, updated_at=now() WHERE id=$1::uuid`, env, isProd)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("ambiente não encontrado")
	}
	_, _ = s.pool.Exec(ctx, `
INSERT INTO auditor_operation_log(username, action, environment_id, resource_id, result)
VALUES($1,'environment.is_production',$2::uuid,$3,'success')`,
		actor, env, fmt.Sprintf("%t", isProd))
	return nil
}
