package analyzer

import (
	"context"
	"fmt"
)

// PerformanceAnalyzer surfaces lock waits, connection pressure and slow query fingerprints.
// Query text is always sanitized (fingerprint only); raw SQL with literals is never stored.
type PerformanceAnalyzer struct{}

func (PerformanceAnalyzer) Name() string { return "performance" }

const (
	lockWaitSecondsThreshold = 30
	connectionUsageThreshold = 0.80 // 80% of max_connections
	slowQueryMeanMsThreshold = 500.0
	slowQueryMinCalls        = 10
)

func (PerformanceAnalyzer) Analyze(_ context.Context, facts SnapshotFacts) ([]Finding, error) {
	out := make([]Finding, 0)

	// Lock waits
	for _, lk := range facts.Locks {
		if lk.Granted || lk.WaitAgeSeconds < lockWaitSecondsThreshold {
			continue
		}
		key := fmt.Sprintf("%s.lock:%s:%d", lk.Database, lk.Mode, lk.PID)
		title := fmt.Sprintf("Long lock wait on %s (%s)", lk.Database, lk.Mode)
		f := Finding{
			EnvironmentID: facts.EnvironmentID,
			AuditRunID:    facts.AuditRunID,
			FindingType:   "performance.lock_wait",
			Severity:      SeverityMedium,
			Status:        StatusOpen,
			Title:         title,
			Summary: fmt.Sprintf(
				"Session waiting %ds for lock mode %s. Review blocking activity; no session termination is suggested.",
				lk.WaitAgeSeconds, lk.Mode,
			),
			ObjectType:   "lock",
			ObjectKey:    key,
			DatabaseName: lk.Database,
			ObjectName:   lk.Relation,
			Evidence: map[string]any{
				"mode":             lk.Mode,
				"wait_age_seconds": lk.WaitAgeSeconds,
				"relation":         lk.Relation,
				"pid":              lk.PID,
				"safety_note":      "Never recommend pg_terminate_backend automatically",
			},
			DedupKey: DedupKey("performance.lock_wait", key, title),
		}
		out = append(out, f)
	}

	// High connection usage
	for _, c := range facts.Connections {
		if c.MaxConnections <= 0 || c.Count <= 0 {
			continue
		}
		ratio := float64(c.Count) / float64(c.MaxConnections)
		if ratio < connectionUsageThreshold {
			continue
		}
		key := fmt.Sprintf("%s.connections", c.Database)
		if c.RoleName != "" {
			key = fmt.Sprintf("%s.connections:%s", c.Database, c.RoleName)
		}
		title := fmt.Sprintf("High connection usage on %s", c.Database)
		sev := SeverityMedium
		if ratio >= 0.95 {
			sev = SeverityHigh
		}
		f := Finding{
			EnvironmentID: facts.EnvironmentID,
			AuditRunID:    facts.AuditRunID,
			FindingType:   "performance.high_connections",
			Severity:      sev,
			Status:        StatusOpen,
			Title:         title,
			Summary: fmt.Sprintf(
				"Connections at %.0f%% of max (%d / %d). Review pooling and idle sessions.",
				ratio*100, c.Count, c.MaxConnections,
			),
			ObjectType:   "database",
			ObjectKey:    key,
			DatabaseName: c.Database,
			ObjectName:   c.RoleName,
			Evidence: map[string]any{
				"count":           c.Count,
				"max_connections": c.MaxConnections,
				"usage_ratio":     ratio,
				"role_name":       c.RoleName,
				"state":           c.State,
			},
			DedupKey: DedupKey("performance.high_connections", key, title),
		}
		out = append(out, f)
	}

	// Slow query fingerprints (sanitized only)
	for _, q := range facts.QueryStats {
		if q.MeanExecTimeMs < slowQueryMeanMsThreshold || q.Calls < slowQueryMinCalls {
			continue
		}
		fp := q.QueryFingerprint
		if fp == "" {
			fp = "unknown"
		}
		key := fmt.Sprintf("%s.query:%s", q.Database, fp)
		title := fmt.Sprintf("Slow query fingerprint on %s", q.Database)
		f := Finding{
			EnvironmentID: facts.EnvironmentID,
			AuditRunID:    facts.AuditRunID,
			FindingType:   "performance.slow_query",
			Severity:      SeverityLow,
			Status:        StatusOpen,
			Title:         title,
			Summary: fmt.Sprintf(
				"Query fingerprint mean exec time %.1f ms over %d calls. Review plan; SQL literals are never exposed.",
				q.MeanExecTimeMs, q.Calls,
			),
			ObjectType:   "query",
			ObjectKey:    key,
			DatabaseName: q.Database,
			ObjectName:   fp,
			Evidence: map[string]any{
				"query_fingerprint":  fp,
				"calls":              q.Calls,
				"mean_exec_time_ms":  q.MeanExecTimeMs,
				"total_exec_time_ms": q.TotalExecTimeMs,
				"rows":               q.Rows,
				"pg_stat_statements": q.PgStatStatements,
				"sanitization_note":  "Only normalized fingerprint stored; no literals or secrets",
			},
			DedupKey: DedupKey("performance.slow_query", key, title),
		}
		out = append(out, f)
	}

	return out, nil
}
