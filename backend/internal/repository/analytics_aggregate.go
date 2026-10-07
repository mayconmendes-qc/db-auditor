package repository

import (
	"context"
	"time"
)

// FindingAggregate contains exact current counts. No UI page limit is applied.
type FindingAggregate struct {
	Total      int
	Open       int
	Critical   int
	High       int
	BySeverity map[string]int
	ByStatus   map[string]int
	ByType     map[string]int
}

// CapabilityAggregate summarizes the latest completed inventory without loading rows.
type CapabilityAggregate struct {
	TotalStorageBytes int64
	Hypertables       int
	JobsScheduled     int
	Policies          int
}

func (s *Store) CountLatestCapabilities(ctx context.Context, environmentID string) (CapabilityAggregate, error) {
	var result CapabilityAggregate
	err := s.pool.QueryRow(ctx, `WITH latest AS (
 SELECT id FROM audit_run WHERE environment_id=$1::uuid
 AND status IN ('success','partial_success')
 ORDER BY started_at DESC,id DESC LIMIT 1
)
SELECT
 COALESCE((SELECT sum(size_bytes) FROM database_snapshot WHERE audit_run_id=(SELECT id FROM latest) AND NOT is_template),0),
 (SELECT count(*) FROM hypertable_snapshot WHERE audit_run_id=(SELECT id FROM latest)),
 (SELECT count(*) FROM job_snapshot WHERE audit_run_id=(SELECT id FROM latest) AND scheduled),
 (SELECT count(*) FROM policy_snapshot WHERE audit_run_id=(SELECT id FROM latest))`, environmentID).
		Scan(&result.TotalStorageBytes, &result.Hypertables, &result.JobsScheduled, &result.Policies)
	return result, err
}

func (s *Store) CountFindings(ctx context.Context, environmentID string) (FindingAggregate, error) {
	result := FindingAggregate{
		BySeverity: map[string]int{},
		ByStatus:   map[string]int{},
		ByType:     map[string]int{},
	}
	rows, err := s.pool.Query(ctx, `SELECT lower(severity), lower(status), finding_type, count(*)
FROM finding WHERE ($1='' OR environment_id=$1::uuid)
GROUP BY lower(severity), lower(status), finding_type`, environmentID)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var severity, status, findingType string
		var count int
		if err := rows.Scan(&severity, &status, &findingType, &count); err != nil {
			return result, err
		}
		result.Total += count
		result.BySeverity[severity] += count
		result.ByStatus[status] += count
		result.ByType[findingType] += count
		if status == "open" || status == "acknowledged" {
			result.Open += count
		}
		if severity == "critical" {
			result.Critical += count
		}
		if severity == "high" {
			result.High += count
		}
	}
	return result, rows.Err()
}

// CountRecentRuns preserves the dashboard's "last 50 runs" definition.
func (s *Store) CountRecentRuns(ctx context.Context, environmentID string) (successful, failed int, err error) {
	err = s.pool.QueryRow(ctx, `WITH recent AS (
  SELECT status FROM audit_run
  WHERE ($1='' OR environment_id=$1::uuid)
  ORDER BY started_at DESC,id DESC LIMIT 50
)
SELECT count(*) FILTER (WHERE status IN ('success','partial_success')),
       count(*) FILTER (WHERE status='failed') FROM recent`, environmentID).Scan(&successful, &failed)
	return
}

// RunTrendPoint is one representative completed run per UTC calendar bucket.
// A nil SizeBytes means that the run had no database inventory.
type RunTrendPoint struct {
	EnvironmentID    string    `json:"environment_id"`
	EnvironmentName  string    `json:"environment_name"`
	AuditRunID       string    `json:"audit_run_id"`
	At               time.Time `json:"at"`
	Status           string    `json:"status"`
	Profile          string    `json:"profile"`
	CollectorVersion string    `json:"collector_version"`
	RuleVersion      string    `json:"rule_version,omitempty"`
	Coverage         string    `json:"coverage"`
	Comparable       bool      `json:"comparable"`
	ComparisonNote   string    `json:"comparison_note,omitempty"`
	SizeBytes        *int64    `json:"size_bytes,omitempty"`
	Findings         *int      `json:"findings,omitempty"`
	Critical         *int      `json:"critical,omitempty"`
	High             *int      `json:"high,omitempty"`
	Score            *int      `json:"score,omitempty"`
	ScoreVersion     string    `json:"score_version,omitempty"`
	ScoreConfidence  float64   `json:"score_confidence,omitempty"`
	CounterReset     bool      `json:"counter_reset,omitempty"`
}

type StorageConsumer struct {
	DatabaseName string
	SchemaName   string
	TableName    string
	SizeBytes    int64
}

func (s *Store) ListTopTableConsumers(ctx context.Context, environmentID string, limit int) ([]StorageConsumer, error) {
	if limit < 1 || limit > 100 {
		limit = 15
	}
	rows, err := s.pool.Query(ctx, `WITH latest AS (
 SELECT id FROM audit_run WHERE environment_id=$1::uuid
 AND status IN ('success','partial_success')
 ORDER BY started_at DESC,id DESC LIMIT 1
)
SELECT database_name,schema_name,table_name,total_size_bytes
FROM table_snapshot WHERE audit_run_id=(SELECT id FROM latest)
ORDER BY total_size_bytes DESC,database_name,schema_name,table_name LIMIT $2`, environmentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []StorageConsumer{}
	for rows.Next() {
		var item StorageConsumer
		if err := rows.Scan(&item.DatabaseName, &item.SchemaName, &item.TableName, &item.SizeBytes); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// ListStorageTrend uses the newest terminal run in each bucket. Keeping
// partial runs visible prevents an apparent gap from masquerading as zero.
func (s *Store) ListStorageTrend(ctx context.Context, environmentID string, from, to time.Time, granularity string) ([]RunTrendPoint, error) {
	rows, err := s.pool.Query(ctx, `WITH ranked AS (
 SELECT ar.id, ar.environment_id, e.name, ar.status, ar.started_at,ar.profile,ar.collector_version,
 COALESCE(NULLIF(a.rule_manifest_hash,''),a.analyzer_version,'') AS rule_version,
 CASE WHEN ar.status='success' AND NOT EXISTS(SELECT 1 FROM audit_run_coverage c WHERE c.audit_run_id=ar.id AND c.status IN ('failed','skipped','attempted')) THEN 'complete' ELSE 'partial' END AS coverage,
 row_number() OVER (PARTITION BY ar.environment_id,
   date_trunc($4, ar.started_at AT TIME ZONE 'UTC')
   ORDER BY ar.started_at DESC, ar.id DESC) AS position
 FROM audit_run ar JOIN audit_environment e ON e.id=ar.environment_id LEFT JOIN analysis_run a ON a.audit_run_id=ar.id
 WHERE ar.status IN ('success','partial_success')
   AND ar.started_at >= $2 AND ar.started_at < $3
   AND ($1='' OR ar.environment_id=$1::uuid)
)
SELECT r.environment_id::text, r.name, r.id::text, r.started_at, r.status,r.profile,r.collector_version,r.rule_version,r.coverage,
       CASE WHEN count(d.id)=0 THEN NULL ELSE sum(d.size_bytes)::bigint END
FROM ranked r LEFT JOIN database_snapshot d ON d.audit_run_id=r.id AND NOT d.is_template
WHERE r.position=1
GROUP BY r.environment_id,r.name,r.id,r.started_at,r.status,r.profile,r.collector_version,r.rule_version,r.coverage
ORDER BY r.started_at,r.environment_id`, environmentID, from, to, granularity)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RunTrendPoint{}
	for rows.Next() {
		var p RunTrendPoint
		if err := rows.Scan(&p.EnvironmentID, &p.EnvironmentName, &p.AuditRunID, &p.At, &p.Status, &p.Profile, &p.CollectorVersion, &p.RuleVersion, &p.Coverage, &p.SizeBytes); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	markTrendComparability(out)
	if err := s.markTrendResets(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListFindingTrend only includes runs with a successful analysis. A completed
// analysis with no observed events is a genuine zero; missing analysis is a gap.
func (s *Store) ListFindingTrend(ctx context.Context, environmentID string, from, to time.Time, granularity string) ([]RunTrendPoint, error) {
	rows, err := s.pool.Query(ctx, `WITH ranked AS (
 SELECT ar.id, ar.environment_id, e.name, ar.status, ar.started_at,ar.profile,ar.collector_version,
 COALESCE(NULLIF(a.rule_manifest_hash,''),a.analyzer_version,'') AS rule_version,
 CASE WHEN ar.status='success' AND NOT EXISTS(SELECT 1 FROM audit_run_coverage c WHERE c.audit_run_id=ar.id AND c.status IN ('failed','skipped','attempted')) THEN 'complete' ELSE 'partial' END AS coverage,
 row_number() OVER (PARTITION BY ar.environment_id,
   date_trunc($4, ar.started_at AT TIME ZONE 'UTC')
   ORDER BY ar.started_at DESC, ar.id DESC) AS position
 FROM audit_run ar JOIN audit_environment e ON e.id=ar.environment_id
 JOIN analysis_run a ON a.audit_run_id=ar.id AND a.status='success'
 WHERE ar.status IN ('success','partial_success')
   AND ar.started_at >= $2 AND ar.started_at < $3
   AND ($1='' OR ar.environment_id=$1::uuid)
)
SELECT r.environment_id::text, r.name, r.id::text, r.started_at, r.status,r.profile,r.collector_version,r.rule_version,r.coverage,
 count(f.id)::integer,
 count(f.id) FILTER (WHERE f.severity='critical')::integer,
 count(f.id) FILTER (WHERE f.severity='high')::integer
FROM ranked r LEFT JOIN finding_event f ON f.audit_run_id=r.id AND f.event_type='observed'
WHERE r.position=1
GROUP BY r.environment_id,r.name,r.id,r.started_at,r.status,r.profile,r.collector_version,r.rule_version,r.coverage
ORDER BY r.started_at,r.environment_id`, environmentID, from, to, granularity)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RunTrendPoint{}
	for rows.Next() {
		var p RunTrendPoint
		if err := rows.Scan(&p.EnvironmentID, &p.EnvironmentName, &p.AuditRunID, &p.At, &p.Status, &p.Profile, &p.CollectorVersion, &p.RuleVersion, &p.Coverage, &p.Findings, &p.Critical, &p.High); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	markTrendComparability(out)
	if err := s.markTrendResets(ctx, out); err != nil {
		return nil, err
	}
	if err := s.addTrendScores(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func markTrendComparability(points []RunTrendPoint) {
	previous := map[string]RunTrendPoint{}
	for i := range points {
		point := &points[i]
		point.Comparable = point.Coverage == "complete"
		if !point.Comparable {
			point.ComparisonNote = "Coleta parcial"
		}
		if prior, ok := previous[point.EnvironmentID]; ok && point.Comparable {
			switch {
			case prior.Coverage != "complete":
				point.Comparable = false
				point.ComparisonNote = "Coleta anterior parcial"
			case prior.Profile != point.Profile:
				point.Comparable = false
				point.ComparisonNote = "Perfil de coleta diferente"
			case prior.CollectorVersion != point.CollectorVersion:
				point.Comparable = false
				point.ComparisonNote = "Versão do coletor diferente"
			case prior.RuleVersion != point.RuleVersion:
				point.Comparable = false
				point.ComparisonNote = "Versão das regras diferente"
			}
		}
		previous[point.EnvironmentID] = *point
	}
}
