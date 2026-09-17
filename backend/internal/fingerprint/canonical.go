// Package fingerprint builds canonical object identities and structural hashes.
package fingerprint

import (
	"fmt"
	"strings"
)

// AlgorithmVersion identifies the fingerprint algorithm for persisted hashes.
const AlgorithmVersion = "1.0.0"

// ObjectType enumerates supported canonical object kinds.
type ObjectType string

const (
	ObjectTypeDatabase   ObjectType = "database"
	ObjectTypeSchema     ObjectType = "schema"
	ObjectTypeTable      ObjectType = "table"
	ObjectTypeColumn     ObjectType = "column"
	ObjectTypeIndex      ObjectType = "index"
	ObjectTypeConstraint ObjectType = "constraint"
	ObjectTypeView       ObjectType = "view"
	ObjectTypeMatView    ObjectType = "materialized_view"
	ObjectTypeFunction   ObjectType = "function"
	ObjectTypeProcedure  ObjectType = "procedure"
	ObjectTypeExtension  ObjectType = "extension"
	ObjectTypeHypertable ObjectType = "hypertable"
	ObjectTypeDimension  ObjectType = "dimension"
	ObjectTypeChunk      ObjectType = "chunk"
	ObjectTypeCAGG       ObjectType = "continuous_aggregate"
	ObjectTypeJob        ObjectType = "job"
	ObjectTypePolicy     ObjectType = "policy"
)

// CanonicalObjectID is the stable identity of an inventoried object.
// Format: environment / server / database / schema / object_type / object_name
type CanonicalObjectID struct {
	Environment string     `json:"environment"`
	Server      string     `json:"server"`
	Database    string     `json:"database"`
	Schema      string     `json:"schema"`
	ObjectType  ObjectType `json:"object_type"`
	ObjectName  string     `json:"object_name"`
}

// String returns the slash-separated canonical form.
func (c CanonicalObjectID) String() string {
	return strings.Join([]string{
		NormalizeName(c.Environment),
		NormalizeName(c.Server),
		NormalizeName(c.Database),
		NormalizeName(c.Schema),
		string(c.ObjectType),
		NormalizeName(c.ObjectName),
	}, "/")
}

// NewCanonical builds a CanonicalObjectID with normalized components.
func NewCanonical(environment, server, database, schema string, objectType ObjectType, objectName string) CanonicalObjectID {
	return CanonicalObjectID{
		Environment: NormalizeName(environment),
		Server:      NormalizeName(server),
		Database:    NormalizeName(database),
		Schema:      NormalizeName(schema),
		ObjectType:  objectType,
		ObjectName:  NormalizeName(objectName),
	}
}

// NormalizeName lowercases and trims identifiers for stable comparison.
func NormalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// NormalizeType maps common PostgreSQL type aliases to a stable form.
func NormalizeType(dataType string) string {
	t := strings.ToLower(strings.TrimSpace(dataType))
	switch t {
	case "int4", "integer":
		return "integer"
	case "int8", "bigint":
		return "bigint"
	case "int2", "smallint":
		return "smallint"
	case "bool", "boolean":
		return "boolean"
	case "float8", "double precision":
		return "double precision"
	case "float4", "real":
		return "real"
	case "timestamptz", "timestamp with time zone":
		return "timestamptz"
	case "timestamp", "timestamp without time zone":
		return "timestamp"
	case "varchar", "character varying":
		return "varchar"
	case "bpchar", "character":
		return "character"
	default:
		return t
	}
}

// NormalizeInterval normalizes interval literals (whitespace and case).
func NormalizeInterval(interval string) string {
	s := strings.ToLower(strings.TrimSpace(interval))
	s = strings.Join(strings.Fields(s), " ")
	return s
}

// Validate reports whether the canonical ID has the required parts for its type.
func (c CanonicalObjectID) Validate() error {
	if c.Environment == "" || c.Database == "" || c.ObjectType == "" || c.ObjectName == "" {
		return fmt.Errorf("canonical id requires environment, database, object_type and object_name")
	}
	needsSchema := c.ObjectType != ObjectTypeDatabase && c.ObjectType != ObjectTypeExtension
	if needsSchema && c.Schema == "" {
		return fmt.Errorf("canonical id for %s requires schema", c.ObjectType)
	}
	return nil
}
