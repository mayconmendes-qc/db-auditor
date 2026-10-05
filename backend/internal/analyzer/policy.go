package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// PolicyAnalyzer diagnoses policies and background jobs.
type PolicyAnalyzer struct{}

func (PolicyAnalyzer) Name() string { return "policy" }

func (PolicyAnalyzer) Analyze(_ context.Context, facts SnapshotFacts) ([]Finding, error) {
	out := make([]Finding, 0)

	// Failed policies / jobs.
	for _, p := range facts.Policies {
		if !isFailedStatus(p.LastRunStatus) {
			continue
		}
		key := fmt.Sprintf("%s.policy.%d", p.Database, p.JobID)
		title := fmt.Sprintf("Policy job failing: %s #%d (%s)", p.Database, p.JobID, p.PolicyType)
		ev := map[string]any{}
		ev["job_id"] = p.JobID
		ev["policy_type"] = p.PolicyType
		ev["last_run_status"] = p.LastRunStatus
		ev["schedule_interval"] = p.ScheduleInterval
		ev["proc_name"] = p.ProcName
		f := Finding{
			EnvironmentID: facts.EnvironmentID,
			AuditRunID:    facts.AuditRunID,
			FindingType:   "policy.job_failed",
			Severity:      SeverityHigh,
			Status:        StatusOpen,
			Title:         title,
			Summary:       fmt.Sprintf("Last run status is %q for %s policy on %s.%s.", p.LastRunStatus, p.PolicyType, p.HypertableSchema, p.HypertableName),
			ObjectType:    "policy",
			ObjectKey:     key,
			DatabaseName:  p.Database,
			SchemaName:    p.HypertableSchema,
			ObjectName:    p.HypertableName,
			Evidence:      ev,
			DedupKey:      DedupKey("policy.job_failed", key, title),
		}
		out = append(out, f)
	}
	for _, j := range facts.Jobs {
		if !isFailedStatus(j.LastRunStatus) && j.TotalFailures == 0 {
			continue
		}
		if !isFailedStatus(j.LastRunStatus) && j.TotalFailures < 3 {
			continue
		}
		key := fmt.Sprintf("%s.job.%d", j.Database, j.JobID)
		title := fmt.Sprintf("Background job unhealthy: %s #%d", j.Database, j.JobID)
		ev := map[string]any{}
		ev["job_id"] = j.JobID
		ev["application"] = j.Application
		ev["last_run_status"] = j.LastRunStatus
		ev["total_failures"] = j.TotalFailures
		ev["scheduled"] = j.Scheduled
		f := Finding{
			EnvironmentID: facts.EnvironmentID,
			AuditRunID:    facts.AuditRunID,
			FindingType:   "job.unhealthy",
			Severity:      SeverityMedium,
			Status:        StatusOpen,
			Title:         title,
			Summary:       fmt.Sprintf("Job %s reports status=%q failures=%d.", j.ProcName, j.LastRunStatus, j.TotalFailures),
			ObjectType:    "job",
			ObjectKey:     key,
			DatabaseName:  j.Database,
			ObjectName:    j.ProcName,
			Evidence:      ev,
			DedupKey:      DedupKey("job.unhealthy", key, title),
		}
		out = append(out, f)
	}

	// Hypertables without retention policy (coverage gap).
	hasRetention := map[string]bool{}
	hasCompression := map[string]bool{}
	for _, p := range facts.Policies {
		ht := fmt.Sprintf("%s.%s.%s", p.Database, p.HypertableSchema, p.HypertableName)
		switch strings.ToLower(p.PolicyType) {
		case "retention":
			hasRetention[ht] = true
		case "compression", "columnstore":
			hasCompression[ht] = true
		}
	}
	for _, ht := range facts.Hypertables {
		key := fmt.Sprintf("%s.%s.%s", ht.Database, ht.Schema, ht.Name)
		if !hasRetention[key] && ht.SizeBytes > 0 {
			title := fmt.Sprintf("Hypertable without retention policy: %s", key)
			f := Finding{
				EnvironmentID: facts.EnvironmentID,
				AuditRunID:    facts.AuditRunID,
				FindingType:   "policy.missing_retention",
				Severity:      SeverityMedium,
				Status:        StatusOpen,
				Title:         title,
				Summary:       "No retention policy detected. Review growth and storage cost.",
				ObjectType:    "hypertable",
				ObjectKey:     key,
				DatabaseName:  ht.Database,
				SchemaName:    ht.Schema,
				ObjectName:    ht.Name,
				Evidence: map[string]any{
					"size_bytes": ht.SizeBytes,
					"num_chunks": ht.NumChunks,
				},
				DedupKey: DedupKey("policy.missing_retention", key, title),
			}
			out = append(out, f)
		}
		if !hasCompression[key] && ht.SizeBytes > (1<<30) {
			title := fmt.Sprintf("Large hypertable without compression policy: %s", key)
			f := Finding{
				EnvironmentID: facts.EnvironmentID,
				AuditRunID:    facts.AuditRunID,
				FindingType:   "policy.missing_compression",
				Severity:      SeverityLow,
				Status:        StatusOpen,
				Title:         title,
				Summary:       "Hypertable exceeds 1 GiB and has no compression/columnstore policy.",
				ObjectType:    "hypertable",
				ObjectKey:     key,
				DatabaseName:  ht.Database,
				SchemaName:    ht.Schema,
				ObjectName:    ht.Name,
				Evidence: map[string]any{
					"size_bytes": ht.SizeBytes,
				},
				DedupKey: DedupKey("policy.missing_compression", key, title),
			}
			out = append(out, f)
		}
	}
	out = append(out, retentionChunkFindings(facts)...)
	out = append(out, compressionHealthFindings(facts)...)
	out = append(out, jobSLOFindings(facts)...)
	return out, nil
}

func retentionChunkFindings(facts SnapshotFacts) []Finding {
	interval := map[string]string{}
	for _, ht := range facts.Hypertables {
		key := fmt.Sprintf("%s.%s.%s", ht.Database, ht.Schema, ht.Name)
		interval[key] = ht.ChunkInterval
	}
	hasReorder := map[string]bool{}
	var out []Finding
	for _, p := range facts.Policies {
		key := fmt.Sprintf("%s.%s.%s", p.Database, p.HypertableSchema, p.HypertableName)
		if strings.EqualFold(p.PolicyType, "reorder") {
			hasReorder[key] = true
		}
		if !strings.EqualFold(p.PolicyType, "retention") || interval[key] == "" {
			continue
		}
		retention := retentionInterval(p)
		chunk := parseInterval(interval[key])
		keep := parseInterval(retention)
		if chunk <= 0 || keep <= 0 {
			continue
		}
		short := keep < chunk
		late := keep > chunk && keep%chunk != 0
		if !short && !late {
			continue
		}
		title := fmt.Sprintf("Retention does not cover a closed chunk: %s", key)
		summary := fmt.Sprintf("Retention %s is shorter than the chunk interval %s, so a closed chunk is not dropped on the retention boundary. This is not a missing-retention finding.", retention, interval[key])
		if late {
			title = fmt.Sprintf("Retention drops later than the chunk boundary: %s", key)
			summary = fmt.Sprintf("Retention %s is not a multiple of the chunk interval %s, so a chunk is dropped only after it is fully outside the window and data stays longer than configured. This is not a missing-retention finding and does not recommend drop_chunks.", retention, interval[key])
		}
		out = append(out, Finding{
			EnvironmentID: facts.EnvironmentID,
			AuditRunID:    facts.AuditRunID,
			FindingType:   "policy.retention_chunk_mismatch",
			Severity:      SeverityHigh,
			Status:        StatusOpen,
			Title:         title,
			Summary:       summary,
			ObjectType:    "hypertable",
			ObjectKey:     key,
			DatabaseName:  p.Database,
			SchemaName:    p.HypertableSchema,
			ObjectName:    p.HypertableName,
			Evidence: map[string]any{
				"retention": retention, "chunk_interval": interval[key],
			},
			DedupKey: DedupKey("policy.retention_chunk_mismatch", key, title),
		})
	}
	for _, ht := range facts.Hypertables {
		key := fmt.Sprintf("%s.%s.%s", ht.Database, ht.Schema, ht.Name)
		if !ht.ReadsOutsideTime || hasReorder[key] {
			continue
		}
		title := fmt.Sprintf("Reorder policy hypothesis: %s", key)
		out = append(out, Finding{
			EnvironmentID: facts.EnvironmentID,
			AuditRunID:    facts.AuditRunID,
			FindingType:   "policy.reorder_hypothesis",
			Severity:      SeverityLow,
			Status:        StatusOpen,
			Title:         title,
			Summary:       "Workload looks dominated by reads that are not the time column, and there is no reorder policy. Hypothesis only.",
			ObjectType:    "hypertable",
			ObjectKey:     key,
			DatabaseName:  ht.Database,
			SchemaName:    ht.Schema,
			ObjectName:    ht.Name,
			DedupKey:      DedupKey("policy.reorder_hypothesis", key, title),
		})
	}
	return out
}

func compressionHealthFindings(facts SnapshotFacts) []Finding {
	var out []Finding
	for _, ht := range facts.Hypertables {
		if ht.UncompressedClosedChunks < 1 {
			continue
		}
		key := fmt.Sprintf("%s.%s.%s", ht.Database, ht.Schema, ht.Name)
		has := false
		var policy PolicyFact
		for _, p := range facts.Policies {
			if fmt.Sprintf("%s.%s.%s", p.Database, p.HypertableSchema, p.HypertableName) == key &&
				(strings.EqualFold(p.PolicyType, "compression") || strings.EqualFold(p.PolicyType, "columnstore")) {
				has = true
				policy = p
			}
		}
		if !has {
			continue
		}
		title := fmt.Sprintf("Compression policy is not compressing closed chunks: %s", key)
		out = append(out, Finding{
			EnvironmentID: facts.EnvironmentID,
			AuditRunID:    facts.AuditRunID,
			FindingType:   "policy.compression_not_applied",
			Severity:      SeverityMedium,
			Status:        StatusOpen,
			Title:         title,
			Summary:       "A compression or columnstore policy exists, but closed chunks are still uncompressed. This is not the missing-policy finding.",
			ObjectType:    "hypertable",
			ObjectKey:     key,
			DatabaseName:  ht.Database,
			SchemaName:    ht.Schema,
			ObjectName:    ht.Name,
			Evidence: map[string]any{
				"uncompressed_closed_chunks": ht.UncompressedClosedChunks,
				"policy_type":                policy.PolicyType,
				"size_bytes":                 ht.SizeBytes,
			},
			DedupKey: DedupKey("policy.compression_not_applied", key, title),
		})
		if policy.CompressionRatio > 0 && policy.CompressionRatio < 1.2 {
			title = fmt.Sprintf("Poor compression ratio: %s", key)
			out = append(out, Finding{
				EnvironmentID: facts.EnvironmentID, AuditRunID: facts.AuditRunID,
				FindingType: "policy.compression_ratio", Severity: SeverityLow, Status: StatusOpen,
				Title: title, Summary: "Compressed size is close to the uncompressed size.",
				ObjectType: "hypertable", ObjectKey: key, DatabaseName: ht.Database, SchemaName: ht.Schema, ObjectName: ht.Name,
				Evidence: map[string]any{"ratio": policy.CompressionRatio, "before_after": true, "policy_type": policy.PolicyType},
				DedupKey: DedupKey("policy.compression_ratio", key, title),
			})
		}
		segment, order := compressionSettings(policy)
		if segment == "" && order == "" {
			title = fmt.Sprintf("Compression segmentby/orderby hypothesis: %s", key)
			summary := "The compression policy has no segmentby or orderby. Hypothesis only; the auditor does not run ALTER TABLE."
			if columnstoreEra(facts.TimescaleVersion) && strings.EqualFold(policy.PolicyType, "compression") {
				summary += " This server is new enough that Timescale names the feature columnstore; the policy still uses the legacy compression procedure."
			}
			out = append(out, Finding{
				EnvironmentID: facts.EnvironmentID, AuditRunID: facts.AuditRunID,
				FindingType: "policy.compression_settings", Severity: SeverityLow, Status: StatusOpen,
				Title: title, Summary: summary,
				ObjectType: "hypertable", ObjectKey: key, DatabaseName: ht.Database, SchemaName: ht.Schema, ObjectName: ht.Name,
				Evidence: map[string]any{
					"policy_type": policy.PolicyType, "segmentby": segment, "orderby": order,
					"size_bytes": ht.SizeBytes, "uncompressed_closed_chunks": ht.UncompressedClosedChunks,
					"timescale_version": facts.TimescaleVersion,
				},
				DedupKey: DedupKey("policy.compression_settings", key, title),
			})
		}
	}
	emitted := map[string]bool{}
	for _, item := range out {
		emitted[item.FindingType+"|"+item.ObjectKey] = true
	}
	for _, p := range facts.Policies {
		if !strings.EqualFold(p.PolicyType, "compression") && !strings.EqualFold(p.PolicyType, "columnstore") {
			continue
		}
		key := fmt.Sprintf("%s.%s.%s", p.Database, p.HypertableSchema, p.HypertableName)
		if p.CompressionRatio > 0 && p.CompressionRatio < 1.2 && !emitted["policy.compression_ratio|"+key] {
			title := fmt.Sprintf("Poor compression ratio: %s", key)
			out = append(out, Finding{
				EnvironmentID: facts.EnvironmentID, AuditRunID: facts.AuditRunID,
				FindingType: "policy.compression_ratio", Severity: SeverityLow, Status: StatusOpen,
				Title: title, Summary: "Compressed size is close to the uncompressed size.",
				ObjectType: "hypertable", ObjectKey: key, DatabaseName: p.Database, SchemaName: p.HypertableSchema, ObjectName: p.HypertableName,
				Evidence: map[string]any{"ratio": p.CompressionRatio, "before_after": true, "policy_type": p.PolicyType},
				DedupKey: DedupKey("policy.compression_ratio", key, title),
			})
		}
		segment, order := compressionSettings(p)
		if segment == "" && order == "" && !emitted["policy.compression_settings|"+key] {
			title := fmt.Sprintf("Compression segmentby/orderby hypothesis: %s", key)
			summary := "The compression policy has no segmentby or orderby. Hypothesis only; the auditor does not run ALTER TABLE."
			if columnstoreEra(facts.TimescaleVersion) && strings.EqualFold(p.PolicyType, "compression") {
				summary += " This server is new enough that Timescale names the feature columnstore; the policy still uses the legacy compression procedure."
			}
			out = append(out, Finding{
				EnvironmentID: facts.EnvironmentID, AuditRunID: facts.AuditRunID,
				FindingType: "policy.compression_settings", Severity: SeverityLow, Status: StatusOpen,
				Title: title, Summary: summary,
				ObjectType: "hypertable", ObjectKey: key, DatabaseName: p.Database, SchemaName: p.HypertableSchema, ObjectName: p.HypertableName,
				Evidence: map[string]any{"policy_type": p.PolicyType, "segmentby": segment, "orderby": order, "timescale_version": facts.TimescaleVersion},
				DedupKey: DedupKey("policy.compression_settings", key, title),
			})
		}
	}
	return out
}

func jobSLOFindings(facts SnapshotFacts) []Finding {
	var out []Finding
	scheduled := 0
	workers := 0
	for _, j := range facts.Jobs {
		if j.Scheduled {
			scheduled++
		}
		if j.MaxBackgroundWorkers > workers {
			workers = j.MaxBackgroundWorkers
		}
		if j.ScheduledJobCount > scheduled {
			scheduled = j.ScheduledJobCount
		}
		if !durationExceeds(j.LastRunDuration, parseInterval(j.ScheduleInterval)) || parseInterval(j.ScheduleInterval) <= 0 {
			continue
		}
		key := fmt.Sprintf("%s.job.%d", j.Database, j.JobID)
		title := fmt.Sprintf("Job slower than its schedule: %s #%d", j.Database, j.JobID)
		out = append(out, Finding{
			EnvironmentID: facts.EnvironmentID, AuditRunID: facts.AuditRunID,
			FindingType: "job.slo_exceeded", Severity: SeverityHigh, Status: StatusOpen,
			Title:      title,
			Summary:    fmt.Sprintf("Last duration %s is longer than the schedule %s while last_run_status=%s.", j.LastRunDuration, j.ScheduleInterval, j.LastRunStatus),
			ObjectType: "job", ObjectKey: key, DatabaseName: j.Database, ObjectName: j.ProcName,
			Evidence: map[string]any{
				"job_id": j.JobID, "proc": j.ProcName, "last_run_status": j.LastRunStatus,
				"duration": j.LastRunDuration, "schedule_interval": j.ScheduleInterval,
				"next_start": j.NextStart, "total_failures": j.TotalFailures,
			},
			DedupKey: DedupKey("job.slo_exceeded", key, title),
		})
	}
	if workers > 0 && scheduled > workers {
		title := "Timescale background workers are saturated"
		out = append(out, Finding{
			EnvironmentID: facts.EnvironmentID, AuditRunID: facts.AuditRunID,
			FindingType: "job.workers_saturated", Severity: SeverityHigh, Status: StatusOpen,
			Title:      title,
			Summary:    fmt.Sprintf("%d scheduled jobs share %d timescaledb.max_background_workers. This is separate from a failed job.", scheduled, workers),
			ObjectType: "environment", ObjectKey: facts.EnvironmentID,
			Evidence: map[string]any{"scheduled_jobs": scheduled, "max_background_workers": workers},
			DedupKey: DedupKey("job.workers_saturated", facts.EnvironmentID, title),
		})
	}
	return out
}

func compressionSettings(p PolicyFact) (string, string) {
	if p.SegmentBy != "" || p.OrderBy != "" {
		return p.SegmentBy, p.OrderBy
	}
	if p.Config == "" {
		return "", ""
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(p.Config), &raw); err != nil {
		return "", ""
	}
	segment, _ := raw["compress_segmentby"].(string)
	order, _ := raw["compress_orderby"].(string)
	return segment, order
}

func columnstoreEra(version string) bool {
	parts := strings.Split(strings.TrimSpace(version), ".")
	if len(parts) < 2 {
		return false
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return false
	}
	return major > 2 || (major == 2 && minor >= 18)
}

func retentionInterval(p PolicyFact) string {
	if p.Config == "" {
		return ""
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(p.Config), &raw); err != nil {
		return ""
	}
	for _, key := range []string{"drop_after", "compress_after"} {
		if v, ok := raw[key].(string); ok {
			return v
		}
	}
	return ""
}

func parseInterval(value string) time.Duration {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return 0
	}
	var n float64
	var unit string
	if _, err := fmt.Sscanf(value, "%f %s", &n, &unit); err != nil {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
		return 0
	}
	switch {
	case strings.HasPrefix(unit, "min"):
		return time.Duration(n * float64(time.Minute))
	case strings.HasPrefix(unit, "hour"), strings.HasPrefix(unit, "hr"):
		return time.Duration(n * float64(time.Hour))
	case strings.HasPrefix(unit, "day"):
		return time.Duration(n * float64(24*time.Hour))
	case strings.HasPrefix(unit, "sec"):
		return time.Duration(n * float64(time.Second))
	default:
		return 0
	}
}

func durationExceeds(value string, limit time.Duration) bool {
	if limit <= 0 {
		return false
	}
	return parseInterval(value) > limit
}

func isFailedStatus(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "failed", "error", "fatal", "crash":
		return true
	default:
		return false
	}
}
