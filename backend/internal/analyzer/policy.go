package analyzer

import (
	"context"
	"fmt"
	"strings"
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
	return out, nil
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
