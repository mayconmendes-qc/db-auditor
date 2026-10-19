package schemadiff

import (
	"strings"

	"github.com/osmendes/db-auditor/internal/comparison"
)

// ChangeEvent is one object-level change between two complete runs.
// Fingerprints only — never raw SQL definitions.
type ChangeEvent struct {
	ObjectType         string
	ObjectKey          string
	ChangeKind         string // added | removed | modified
	PriorFingerprint   string
	CurrentFingerprint string
	ChangedFields      []string
}

// EventsFromDiff converts a comparison result into timeline change events.
func EventsFromDiff(res comparison.Result) []ChangeEvent {
	out := make([]ChangeEvent, 0, len(res.Objects))
	for _, o := range res.Objects {
		switch o.Status {
		case comparison.StatusOnlyTarget:
			out = append(out, ChangeEvent{
				ObjectType:         o.ObjectType,
				ObjectKey:          o.ObjectKey,
				ChangeKind:         "added",
				CurrentFingerprint: o.TargetFP,
			})
		case comparison.StatusOnlySource:
			out = append(out, ChangeEvent{
				ObjectType:       o.ObjectType,
				ObjectKey:        o.ObjectKey,
				ChangeKind:       "removed",
				PriorFingerprint: o.SourceFP,
			})
		case comparison.StatusDrift:
			fields := make([]string, 0, len(o.FieldDiffs))
			for _, f := range o.FieldDiffs {
				fields = append(fields, f.Field)
			}
			out = append(out, ChangeEvent{
				ObjectType:         o.ObjectType,
				ObjectKey:          o.ObjectKey,
				ChangeKind:         "modified",
				PriorFingerprint:   o.SourceFP,
				CurrentFingerprint: o.TargetFP,
				ChangedFields:      fields,
			})
		}
	}
	return out
}

// OutOfPipelineSeverity maps object/change to base severity (production).
// Callers drop one level when the environment is not marked production.
func OutOfPipelineSeverity(objectType, changeKind string, securityDefiner bool) string {
	ot := strings.ToLower(objectType)
	ck := strings.ToLower(changeKind)
	switch {
	case ot == "table" || ot == "column":
		if ck == "removed" || ck == "modified" {
			return "high"
		}
		return "low"
	case ot == "index":
		return "medium"
	case ot == "function" || ot == "view":
		if securityDefiner {
			return "high"
		}
		return "medium"
	case ck == "added":
		return "low"
	default:
		return "medium"
	}
}

// DowngradeSeverity drops one level (production flag absent).
func DowngradeSeverity(sev string) string {
	switch strings.ToLower(sev) {
	case "critical":
		return "high"
	case "high":
		return "medium"
	case "medium":
		return "low"
	default:
		return "info"
	}
}
