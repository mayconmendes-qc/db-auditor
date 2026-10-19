package dqcontract

import (
	"strings"
	"time"
)

// SnapshotColumn is a column from the latest complete collection.
type SnapshotColumn struct {
	Name       string
	DataType   string
	IsNullable bool
}

// SnapshotConstraint is a PK/unique/FK from the latest complete collection.
type SnapshotConstraint struct {
	Name                string
	Type                string // p, u, f
	ConstrainedColumns  []string
	ReferencedSchema    string
	ReferencedTable     string
	ReferencedColumns   []string
}

// ProposalInput feeds RF-03 generation.
type ProposalInput struct {
	Database    string
	Schema      string
	Table       string
	Columns     []SnapshotColumn
	Constraints []SnapshotConstraint
	SampleLimit int
}

var freshnessNames = map[string]struct{}{
	"updated_at": {}, "atualizado_em": {}, "modified_at": {}, "modificado_em": {},
	"last_updated": {}, "last_modified": {}, "alterado_em": {}, "updatedat": {},
	"modification_time": {}, "data_atualizacao": {}, "dt_atualizacao": {},
}

// ProposeFromSnapshot builds a proposed contract (never auto-approved).
func ProposeFromSnapshot(in ProposalInput) Document {
	limit := in.SampleLimit
	if limit <= 0 {
		limit = DefaultSampleLimit
	}
	doc := Document{
		Database:    in.Database,
		Schema:      in.Schema,
		Table:       in.Table,
		SampleLimit: limit,
		Checks:      nil,
	}
	zero := 0.0
	for _, col := range in.Columns {
		if !col.IsNullable {
			doc.Checks = append(doc.Checks, Check{
				Type:       CheckNulls,
				Column:     col.Name,
				MaxNullPct: &zero,
			})
		}
	}
	seenUnique := map[string]struct{}{}
	for _, c := range in.Constraints {
		switch c.Type {
		case "p", "u":
			if len(c.ConstrainedColumns) == 0 {
				continue
			}
			key := strings.Join(c.ConstrainedColumns, ",")
			if _, ok := seenUnique[key]; ok {
				continue
			}
			seenUnique[key] = struct{}{}
			doc.Checks = append(doc.Checks, Check{
				Type:    CheckUniqueness,
				Columns: append([]string{}, c.ConstrainedColumns...),
			})
		case "f":
			if len(c.ConstrainedColumns) != 1 || len(c.ReferencedColumns) != 1 {
				continue // only simple FKs
			}
			doc.Checks = append(doc.Checks, Check{
				Type:      CheckOrphan,
				Column:    c.ConstrainedColumns[0],
				RefSchema: c.ReferencedSchema,
				RefTable:  c.ReferencedTable,
				RefColumn: c.ReferencedColumns[0],
			})
		}
	}
	freshH := float64(DefaultFreshnessH)
	for _, col := range in.Columns {
		name := strings.ToLower(strings.TrimSpace(col.Name))
		if _, ok := freshnessNames[name]; !ok {
			continue
		}
		if !isTemporalType(col.DataType) {
			continue
		}
		doc.Checks = append(doc.Checks, Check{
			Type:        CheckFreshness,
			Column:      col.Name,
			MaxAgeHours: &freshH,
		})
		break
	}
	if len(doc.Checks) > MaxChecks {
		doc.Checks = doc.Checks[:MaxChecks]
	}
	return doc
}

func isTemporalType(t string) bool {
	t = strings.ToLower(t)
	return strings.HasPrefix(t, "timestamp") || strings.HasPrefix(t, "date") || t == "timestamptz"
}

// IsStaleProposalAge is unused helper reserved for future UI hints.
func IsStaleProposalAge(authoredAt time.Time, maxAge time.Duration) bool {
	if authoredAt.IsZero() || maxAge <= 0 {
		return false
	}
	return time.Since(authoredAt) > maxAge
}
