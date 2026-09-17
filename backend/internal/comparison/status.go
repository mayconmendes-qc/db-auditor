// Package comparison classifies structural differences between snapshot sets.
package comparison

// DriftStatus classifies the relationship of an object between source and target.
type DriftStatus string

const (
	StatusMatch      DriftStatus = "MATCH"
	StatusDrift      DriftStatus = "DRIFT"
	StatusOnlySource DriftStatus = "ONLY_SOURCE"
	StatusOnlyTarget DriftStatus = "ONLY_TARGET"
	StatusUnknown    DriftStatus = "UNKNOWN"
)

// FieldDiff records a single field difference when status is DRIFT.
type FieldDiff struct {
	Field  string `json:"field"`
	Source string `json:"source,omitempty"`
	Target string `json:"target,omitempty"`
}

// ObjectDiff is one compared object pair (or singleton).
type ObjectDiff struct {
	ObjectType string      `json:"object_type"`
	ObjectKey  string      `json:"object_key"`
	Status     DriftStatus `json:"status"`
	SourceName string      `json:"source_name,omitempty"`
	TargetName string      `json:"target_name,omitempty"`
	SourceFP   string      `json:"source_fingerprint,omitempty"`
	TargetFP   string      `json:"target_fingerprint,omitempty"`
	FieldDiffs []FieldDiff `json:"field_diffs,omitempty"`
}

// Summary aggregates counts by status.
type Summary struct {
	Match      int `json:"match"`
	Drift      int `json:"drift"`
	OnlySource int `json:"only_source"`
	OnlyTarget int `json:"only_target"`
	Unknown    int `json:"unknown"`
	Total      int `json:"total"`
}

// Result is the full comparison output.
type Result struct {
	SourceRunID string       `json:"source_run_id,omitempty"`
	TargetRunID string       `json:"target_run_id,omitempty"`
	Summary     Summary      `json:"summary"`
	Objects     []ObjectDiff `json:"objects"`
}

func summarize(objects []ObjectDiff) Summary {
	var s Summary
	for _, o := range objects {
		s.Total++
		switch o.Status {
		case StatusMatch:
			s.Match++
		case StatusDrift:
			s.Drift++
		case StatusOnlySource:
			s.OnlySource++
		case StatusOnlyTarget:
			s.OnlyTarget++
		default:
			s.Unknown++
		}
	}
	return s
}
