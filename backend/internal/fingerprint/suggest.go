package fingerprint

import (
	"strings"
)

// MappingCandidate is a suggested correspondence between source and target objects.
type MappingCandidate struct {
	SourceDatabase    string  `json:"source_database"`
	SourceSchema      string  `json:"source_schema"`
	SourceObjectType  string  `json:"source_object_type"`
	SourceObjectName  string  `json:"source_object_name"`
	TargetDatabase    string  `json:"target_database"`
	TargetSchema      string  `json:"target_schema"`
	TargetObjectType  string  `json:"target_object_type"`
	TargetObjectName  string  `json:"target_object_name"`
	RelationType      string  `json:"relation_type"`
	Confidence        float64 `json:"confidence"`
	SourceFingerprint string  `json:"source_fingerprint,omitempty"`
	TargetFingerprint string  `json:"target_fingerprint,omitempty"`
}

// ObjectRef is a lightweight inventory reference used for suggestions.
type ObjectRef struct {
	Database    string
	Schema      string
	ObjectType  string
	ObjectName  string
	Fingerprint string
}

// SuggestMappings proposes correspondences by name and optional fingerprint equality.
// Typical Unifique→Tiger case: source database maps to target schema under tsdb.
func SuggestMappings(source, target []ObjectRef, targetDefaultDB string) []MappingCandidate {
	if targetDefaultDB == "" {
		targetDefaultDB = "tsdb"
	}
	out := make([]MappingCandidate, 0)
	targetByKey := make(map[string]ObjectRef)
	targetByFP := make(map[string]ObjectRef)
	for _, t := range target {
		k := key(t.Schema, t.ObjectType, t.ObjectName)
		targetByKey[k] = t
		if t.Fingerprint != "" {
			targetByFP[t.Fingerprint] = t
		}
		// also index by database name as schema for Unifique→Tiger
		if t.ObjectType == string(ObjectTypeSchema) || t.ObjectType == string(ObjectTypeDatabase) {
			targetByKey[key("", string(ObjectTypeSchema), t.ObjectName)] = t
		}
	}
	for _, s := range source {
		// database → schema mapping (Unifique DB name equals Tiger schema)
		if s.ObjectType == string(ObjectTypeDatabase) {
			if t, ok := targetByKey[key("", string(ObjectTypeSchema), s.ObjectName)]; ok {
				out = append(out, MappingCandidate{
					SourceDatabase: s.Database, SourceSchema: s.Schema,
					SourceObjectType: s.ObjectType, SourceObjectName: s.ObjectName,
					TargetDatabase: targetDefaultDB, TargetSchema: t.ObjectName,
					TargetObjectType: string(ObjectTypeSchema), TargetObjectName: t.ObjectName,
					RelationType: "database_to_schema", Confidence: 0.85,
					SourceFingerprint: s.Fingerprint, TargetFingerprint: t.Fingerprint,
				})
				continue
			}
		}
		if s.Fingerprint != "" {
			if t, ok := targetByFP[s.Fingerprint]; ok {
				out = append(out, MappingCandidate{
					SourceDatabase: s.Database, SourceSchema: s.Schema,
					SourceObjectType: s.ObjectType, SourceObjectName: s.ObjectName,
					TargetDatabase: t.Database, TargetSchema: t.Schema,
					TargetObjectType: t.ObjectType, TargetObjectName: t.ObjectName,
					RelationType: "identical", Confidence: 0.95,
					SourceFingerprint: s.Fingerprint, TargetFingerprint: t.Fingerprint,
				})
				continue
			}
		}
		k := key(s.Schema, s.ObjectType, s.ObjectName)
		if t, ok := targetByKey[k]; ok {
			rel := "identical"
			conf := 0.7
			if !strings.EqualFold(s.ObjectName, t.ObjectName) {
				rel = "renamed"
				conf = 0.55
			}
			out = append(out, MappingCandidate{
				SourceDatabase: s.Database, SourceSchema: s.Schema,
				SourceObjectType: s.ObjectType, SourceObjectName: s.ObjectName,
				TargetDatabase: t.Database, TargetSchema: t.Schema,
				TargetObjectType: t.ObjectType, TargetObjectName: t.ObjectName,
				RelationType: rel, Confidence: conf,
				SourceFingerprint: s.Fingerprint, TargetFingerprint: t.Fingerprint,
			})
		}
	}
	return out
}

func key(schema, objectType, name string) string {
	return NormalizeName(schema) + "|" + strings.ToLower(objectType) + "|" + NormalizeName(name)
}
