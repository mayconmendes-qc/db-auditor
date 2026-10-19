// Package schemadiff builds comparable object sets from snapshot-store rows
// for real schema diffs between audit runs (spec 04).
package schemadiff

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/osmendes/db-auditor/internal/comparison"
	"github.com/osmendes/db-auditor/internal/fingerprint"
)

// SnapshotObjects holds typed inventory rows for one side of a diff.
// Field values that may contain SQL (defaults, predicates, bodies) must already
// be fingerprinted before BuildItems — never pass raw definitions here.
type SnapshotObjects struct {
	Tables      []TableObj
	Columns     []ColumnObj
	Indexes     []IndexObj
	Constraints []ConstraintObj
	Views       []ViewObj
	Functions   []FunctionObj
	Triggers    []TriggerObj
	Sequences   []SequenceObj
	Hypertables []HypertableObj
	Policies    []PolicyObj
}

type TableObj struct {
	Database, Schema, Name string
	Relkind                string
	HasPrimaryKey          bool
	ColumnCount            int
}

type ColumnObj struct {
	Database, Schema, Table, Name string
	DataType                      string
	IsNullable                    bool
	DefaultFingerprint            string // fingerprint of default expression; empty if none
	IsGenerated                   bool
}

type IndexObj struct {
	Database, Schema, Table, Name string
	DefinitionFingerprint         string
	IsUnique, IsPrimary           bool
	AccessMethod                  string
}

type ConstraintObj struct {
	Database, Schema, Table, Name, Type string
	DefinitionFingerprint               string
}

type ViewObj struct {
	Database, Schema, Name string
	DefinitionFingerprint  string
	Relkind                string
}

type FunctionObj struct {
	Database, Schema, Name, IdentityArgs string
	DefinitionFingerprint                string
	Language                             string
	IsSecurityDefiner                    bool
}

type TriggerObj struct {
	Database, Schema, Table, Name string
	DefinitionFingerprint         string
}

type SequenceObj struct {
	Database, Schema, Name string
	DataType               string
}

type HypertableObj struct {
	Database, Schema, Name string
	NumDimensions          int
	CompressionEnabled     bool
}

type PolicyObj struct {
	Database, Schema, Table, Name, Kind string
	DefinitionFingerprint               string
}

// NameMapping renames a source key to a target key (object_mapping).
type NameMapping struct {
	ObjectType string
	SourceKey  string
	TargetKey  string
}

// BuildItems converts snapshot rows into comparison.ObjectItem values.
// mappings rewrite source keys so differently named objects can pair.
func BuildItems(objs SnapshotObjects, mappings []NameMapping) []comparison.ObjectItem {
	remap := indexMappings(mappings)
	out := make([]comparison.ObjectItem, 0, estimateCount(objs))

	for _, t := range objs.Tables {
		key := applyMap(remap, "table", comparison.KeyFor(t.Database, t.Schema, t.Name))
		out = append(out, comparison.ObjectItem{
			ObjectType:  "table",
			Key:         key,
			Name:        t.Schema + "." + t.Name,
			Fingerprint: hashFields(t.Relkind, strconv.FormatBool(t.HasPrimaryKey), strconv.Itoa(t.ColumnCount)),
			Fields: map[string]string{
				"relkind":         t.Relkind,
				"has_primary_key": strconv.FormatBool(t.HasPrimaryKey),
				"column_count":    strconv.Itoa(t.ColumnCount),
			},
		})
	}
	for _, c := range objs.Columns {
		key := applyMap(remap, "column", comparison.KeyFor(c.Database, c.Schema, c.Table, c.Name))
		fields := map[string]string{
			"data_type":    c.DataType,
			"is_nullable":  strconv.FormatBool(c.IsNullable),
			"is_generated": strconv.FormatBool(c.IsGenerated),
		}
		if c.DefaultFingerprint != "" {
			fields["default_fingerprint"] = c.DefaultFingerprint
		}
		out = append(out, comparison.ObjectItem{
			ObjectType:  "column",
			Key:         key,
			Name:        c.Schema + "." + c.Table + "." + c.Name,
			Fingerprint: hashFields(c.DataType, fields["is_nullable"], c.DefaultFingerprint, fields["is_generated"]),
			Fields:      fields,
		})
	}
	for _, i := range objs.Indexes {
		key := applyMap(remap, "index", comparison.KeyFor(i.Database, i.Schema, i.Table, i.Name))
		out = append(out, comparison.ObjectItem{
			ObjectType:  "index",
			Key:         key,
			Name:        i.Schema + "." + i.Name,
			Fingerprint: i.DefinitionFingerprint,
			Fields: map[string]string{
				"is_unique":     strconv.FormatBool(i.IsUnique),
				"is_primary":    strconv.FormatBool(i.IsPrimary),
				"access_method": i.AccessMethod,
				"definition_fp": i.DefinitionFingerprint,
			},
		})
	}
	for _, c := range objs.Constraints {
		key := applyMap(remap, "constraint", comparison.KeyFor(c.Database, c.Schema, c.Table, c.Name))
		out = append(out, comparison.ObjectItem{
			ObjectType:  "constraint",
			Key:         key,
			Name:        c.Schema + "." + c.Name,
			Fingerprint: c.DefinitionFingerprint,
			Fields: map[string]string{
				"type":          c.Type,
				"definition_fp": c.DefinitionFingerprint,
			},
		})
	}
	for _, v := range objs.Views {
		key := applyMap(remap, "view", comparison.KeyFor(v.Database, v.Schema, v.Name))
		out = append(out, comparison.ObjectItem{
			ObjectType:  "view",
			Key:         key,
			Name:        v.Schema + "." + v.Name,
			Fingerprint: v.DefinitionFingerprint,
			Fields: map[string]string{
				"relkind":       v.Relkind,
				"definition_fp": v.DefinitionFingerprint,
			},
		})
	}
	for _, f := range objs.Functions {
		key := applyMap(remap, "function", comparison.KeyFor(f.Database, f.Schema, f.Name, f.IdentityArgs))
		out = append(out, comparison.ObjectItem{
			ObjectType:  "function",
			Key:         key,
			Name:        f.Schema + "." + f.Name + "(" + f.IdentityArgs + ")",
			Fingerprint: f.DefinitionFingerprint,
			Fields: map[string]string{
				"language":            f.Language,
				"is_security_definer": strconv.FormatBool(f.IsSecurityDefiner),
				"definition_fp":       f.DefinitionFingerprint,
			},
		})
	}
	for _, t := range objs.Triggers {
		key := applyMap(remap, "trigger", comparison.KeyFor(t.Database, t.Schema, t.Table, t.Name))
		out = append(out, comparison.ObjectItem{
			ObjectType:  "trigger",
			Key:         key,
			Name:        t.Schema + "." + t.Table + "." + t.Name,
			Fingerprint: t.DefinitionFingerprint,
		})
	}
	for _, s := range objs.Sequences {
		key := applyMap(remap, "sequence", comparison.KeyFor(s.Database, s.Schema, s.Name))
		out = append(out, comparison.ObjectItem{
			ObjectType:  "sequence",
			Key:         key,
			Name:        s.Schema + "." + s.Name,
			Fingerprint: hashFields(s.DataType),
			Fields:      map[string]string{"data_type": s.DataType},
		})
	}
	for _, h := range objs.Hypertables {
		key := applyMap(remap, "hypertable", comparison.KeyFor(h.Database, h.Schema, h.Name))
		out = append(out, comparison.ObjectItem{
			ObjectType:  "hypertable",
			Key:         key,
			Name:        h.Schema + "." + h.Name,
			Fingerprint: hashFields(strconv.Itoa(h.NumDimensions), strconv.FormatBool(h.CompressionEnabled)),
			Fields: map[string]string{
				"num_dimensions":      strconv.Itoa(h.NumDimensions),
				"compression_enabled": strconv.FormatBool(h.CompressionEnabled),
			},
		})
	}
	for _, p := range objs.Policies {
		key := applyMap(remap, "policy", comparison.KeyFor(p.Database, p.Schema, p.Table, p.Name))
		out = append(out, comparison.ObjectItem{
			ObjectType:  "policy",
			Key:         key,
			Name:        p.Schema + "." + p.Table + "." + p.Name,
			Fingerprint: p.DefinitionFingerprint,
			Fields: map[string]string{
				"kind":          p.Kind,
				"definition_fp": p.DefinitionFingerprint,
			},
		})
	}
	return out
}

// Diff compares two sides after optional name mapping on the source.
func Diff(source, target SnapshotObjects, mappings []NameMapping) comparison.Result {
	return comparison.CompareSets(BuildItems(source, mappings), BuildItems(target, nil))
}

// FingerprintValue hashes a definition-like string the same way detail views do.
// Empty input yields empty fingerprint (no default / no body).
func FingerprintValue(value string) string {
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(fingerprint.NormalizeSQL(value)))
	return hex.EncodeToString(sum[:])
}

func estimateCount(o SnapshotObjects) int {
	return len(o.Tables) + len(o.Columns) + len(o.Indexes) + len(o.Constraints) +
		len(o.Views) + len(o.Functions) + len(o.Triggers) + len(o.Sequences) +
		len(o.Hypertables) + len(o.Policies)
}

func indexMappings(mappings []NameMapping) map[string]string {
	m := make(map[string]string, len(mappings))
	for _, item := range mappings {
		k := strings.ToLower(item.ObjectType) + "|" + fingerprint.NormalizeName(item.SourceKey)
		m[k] = item.TargetKey
	}
	return m
}

func applyMap(remap map[string]string, objectType, key string) string {
	if mapped, ok := remap[strings.ToLower(objectType)+"|"+fingerprint.NormalizeName(key)]; ok {
		return mapped
	}
	return key
}

func hashFields(parts ...string) string {
	return FingerprintValue(strings.Join(parts, "\x1f"))
}

// Qualify is a helper for tests and callers building keys.
func Qualify(parts ...string) string {
	return comparison.KeyFor(parts...)
}

// ErrMechanismMismatch is returned by API layers when engines differ.
var ErrMechanismMismatch = fmt.Errorf("schema diff requires the same database mechanism on both sides")
