package analyzer

import (
	"context"
	"fmt"
)

// Default inactivity window: 90 days without observed DML.
const inactivityDaysThreshold = 90

// InactivityAnalyzer flags objects that may be inactive.
// Classification is always POSSIBLY_INACTIVE — never recommends DROP/TRUNCATE.
type InactivityAnalyzer struct{}

func (InactivityAnalyzer) Name() string { return "inactivity" }

func (InactivityAnalyzer) Analyze(_ context.Context, facts SnapshotFacts) ([]Finding, error) {
	out := make([]Finding, 0)
	for _, a := range facts.Activity {
		if a.DaysSinceDML < inactivityDaysThreshold && a.MaxTimeValue == nil {
			continue
		}
		// Prefer explicit days_since_dml; fall back to max time age if provided via DaysSinceDML.
		days := a.DaysSinceDML
		if days < inactivityDaysThreshold {
			continue
		}
		// Skip empty objects with zero tuples — still only POSSIBLY_INACTIVE.
		key := fmt.Sprintf("%s.%s.%s", a.Database, a.Schema, a.Name)
		title := fmt.Sprintf("Possibly inactive %s: %s", a.ObjectType, key)
		f := Finding{
			EnvironmentID: facts.EnvironmentID,
			AuditRunID:    facts.AuditRunID,
			FindingType:   "inactivity.possibly_inactive",
			Severity:      SeverityLow,
			Status:        StatusOpen,
			Title:         title,
			Summary: fmt.Sprintf(
				"No significant DML observed for ~%d days. Classified as POSSIBLY_INACTIVE only — never auto-delete.",
				days,
			),
			ObjectType:   a.ObjectType,
			ObjectKey:    key,
			DatabaseName: a.Database,
			SchemaName:   a.Schema,
			ObjectName:   a.Name,
			Evidence: map[string]any{
				"days_since_dml": days,
				"threshold_days": inactivityDaysThreshold,
				"n_live_tup":     a.NLiveTup,
				"n_tup_ins":      a.NTupIns,
				"n_tup_upd":      a.NTupUpd,
				"n_tup_del":      a.NTupDel,
				"classification": "POSSIBLY_INACTIVE",
				"safety_note":    "Never recommend DROP, TRUNCATE or automatic deletion",
			},
			DedupKey: DedupKey("inactivity.possibly_inactive", key, title),
		}
		if a.LastDataChange != nil {
			f.Evidence["last_data_change"] = a.LastDataChange.UTC().Format("2006-01-02T15:04:05Z")
		}
		if a.MaxTimeValue != nil {
			f.Evidence["max_time_value"] = a.MaxTimeValue.UTC().Format("2006-01-02T15:04:05Z")
		}
		out = append(out, f)
	}
	return out, nil
}
