package analyzer

import (
	"context"
	"fmt"
	"time"
)

// RestoreDrillFact is the latest successful target restore drill (plan 08).
type RestoreDrillFact struct {
	ID          string
	Succeeded   bool
	RTOSeconds  *int
	PerformedAt time.Time
	DivergenceN int
	ArtifactSHA string
}

// RestorePolicyFact is the per-environment restore policy.
type RestorePolicyFact struct {
	RTOTargetSeconds int
	MaxAgeDays       int
}

// BackupAnalyzer emits backup.* findings from restore-drill facts (RF-08).
type BackupAnalyzer struct{}

func (BackupAnalyzer) Name() string { return "backup" }

func (BackupAnalyzer) Analyze(_ context.Context, facts SnapshotFacts) ([]Finding, error) {
	out := make([]Finding, 0)
	pol := facts.RestorePolicy
	if pol == nil {
		pol = &RestorePolicyFact{RTOTargetSeconds: 600, MaxAgeDays: 30}
	}
	maxAge := time.Duration(pol.MaxAgeDays) * 24 * time.Hour
	if maxAge <= 0 {
		maxAge = 30 * 24 * time.Hour
	}

	last := facts.LastRestoreDrill
	now := time.Now().UTC()
	overdue := last == nil || !last.Succeeded || now.Sub(last.PerformedAt) > maxAge
	if overdue {
		days := pol.MaxAgeDays
		if days <= 0 {
			days = 30
		}
		title := "Ensaio de restauração do banco auditado em atraso"
		summary := fmt.Sprintf("Sem ensaio bem-sucedido de target_restore_drill nos últimos %d dias.", days)
		ev := map[string]any{"max_age_days": days, "threshold_days": days}
		if last != nil {
			ev["last_performed_at"] = last.PerformedAt.UTC().Format(time.RFC3339)
			ev["last_succeeded"] = last.Succeeded
		}
		out = append(out, Finding{
			EnvironmentID: facts.EnvironmentID,
			AuditRunID:    facts.AuditRunID,
			FindingType:   "backup.drill_overdue",
			Severity:      SeverityMedium,
			Status:        StatusOpen,
			Title:         title,
			Summary:       summary,
			ObjectType:    "environment",
			ObjectKey:     facts.EnvironmentID,
			Evidence:      ev,
			DedupKey:      DedupKey("backup.drill_overdue", facts.EnvironmentID, title),
		})
	}

	if last != nil && last.RTOSeconds != nil && pol.RTOTargetSeconds > 0 && *last.RTOSeconds > pol.RTOTargetSeconds {
		title := "RTO do ensaio de restauração excedeu o alvo"
		out = append(out, Finding{
			EnvironmentID: facts.EnvironmentID,
			AuditRunID:    facts.AuditRunID,
			FindingType:   "backup.rto_exceeded",
			Severity:      SeverityHigh,
			Status:        StatusOpen,
			Title:         title,
			Summary:       "RTO medido no último ensaio bem-sucedido está acima do RTO-alvo do ambiente.",
			ObjectType:    "environment",
			ObjectKey:     facts.EnvironmentID,
			Evidence: map[string]any{
				"rto_seconds":        *last.RTOSeconds,
				"rto_target_seconds": pol.RTOTargetSeconds,
				"drill_id":           last.ID,
			},
			DedupKey: DedupKey("backup.rto_exceeded", facts.EnvironmentID, title),
		})
	}
	return out, nil
}
