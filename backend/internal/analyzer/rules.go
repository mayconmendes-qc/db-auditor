package analyzer

import (
	"encoding/json"
	"sort"
	"strings"
)

// RuleDefinition is immutable for a given (ID, Version). A future change in
// thresholds or interpretation must create a new version, not rewrite history.
type RuleDefinition struct {
	ID                string         `json:"rule_id"`
	Version           string         `json:"rule_version"`
	Category          string         `json:"category"`
	Confidence        float64        `json:"confidence"`
	Impact            string         `json:"impact"`
	Risk              string         `json:"risk"`
	Recommendation    string         `json:"recommendation"`
	Validation        string         `json:"validation"`
	References        []string       `json:"references"`
	DefaultParameters map[string]any `json:"default_parameters"`
}

// RulePolicy is an environment or schema override loaded from the internal
// store. Schema-specific policies take precedence over environment defaults.
type RulePolicy struct {
	EnvironmentID string         `json:"environment_id"`
	SchemaName    string         `json:"schema_name"`
	RuleID        string         `json:"rule_id"`
	Enabled       bool           `json:"enabled"`
	Parameters    map[string]any `json:"parameters"`
}

var ruleIDs = []string{
	"storage.large_table", "storage.top_consumer", "index.unused", "index.overlap",
	"chunk.high_count", "chunk.size_skew", "cagg.missing_refresh_policy", "cagg.overlap",
	"policy.missing_retention", "policy.missing_compression", "policy.job_failed", "job.unhealthy",
	"inactivity.possibly_inactive", "vacuum.high_dead_tuples", "performance.lock_wait",
	"performance.high_connections", "performance.slow_query", "performance.workload_scan",
	"performance.workload_write", "performance.workload_cost", "security.excessive_privilege",
	"security.powerful_role", "security.security_definer", "integrity.missing_primary_key",
	"integrity.fk_without_index", "integrity.constraint_unvalidated", "integrity.fk_type_mismatch",
	"integrity.orphan_sequence", "integrity.sequence_default_mismatch", "index.prefix_overlap", "index.invalid", "index.write_burden",
	"index.investigate_missing", "maintenance.stale_analyze", "maintenance.dead_tuple_pressure",
	"model.wide_table", "model.repeated_columns", "model.duplicate_entity",
	"model.implicit_relationship", "model.naming_inconsistent", "model.undocumented_critical",
	"model.type_review", "model.jsonb_critical",
}

// Catalog is the versioned, golden-tested source of rule metadata.
func Catalog() []RuleDefinition {
	out := make([]RuleDefinition, 0, len(ruleIDs))
	for _, id := range ruleIDs {
		category := strings.SplitN(id, ".", 2)[0]
		confidence := 0.90
		risk := "low"
		if category == "model" || id == "index.prefix_overlap" || id == "index.investigate_missing" || id == "integrity.fk_type_mismatch" {
			confidence = 0.45
			risk = "hypothesis"
		}
		if category == "security" {
			confidence = 0.80
			risk = "security_review"
		}
		impact := "Operational or structural quality"
		switch category {
		case "integrity":
			impact = "Potential referential-integrity or identity risk"
		case "index", "performance":
			impact = "Potential query latency or write overhead"
		case "maintenance", "vacuum":
			impact = "Potential storage or planner-statistics degradation"
		case "security":
			impact = "Potential privilege or execution risk"
		case "model":
			impact = "Possible maintainability concern (hypothesis)"
		}
		parameters := map[string]any{}
		switch id {
		case "model.wide_table":
			parameters["min_columns"] = 50
		case "model.undocumented_critical":
			parameters["min_bytes"] = float64(1 << 30)
		case "index.write_burden":
			parameters["min_indexes"] = 8
			parameters["min_writes"] = 100000
		case "maintenance.stale_analyze":
			parameters["max_age_days"] = 30
		case "maintenance.dead_tuple_pressure":
			parameters["min_dead_tuples"] = 10000
			parameters["min_dead_ratio"] = 0.2
		case "index.investigate_missing":
			parameters["min_calls"] = 100
			parameters["min_shared_reads"] = 1000
		case "model.type_review":
			parameters["check_money"] = true
			parameters["check_timestamp_without_timezone"] = true
			parameters["check_status_text"] = true
		case "model.naming_inconsistent":
			parameters["convention"] = "snake_case"
		}
		out = append(out, RuleDefinition{
			ID: id, Version: "1.0.0", Category: category, Confidence: confidence,
			Impact: impact, Risk: risk,
			Recommendation: ruleRecommendation(id),
			Validation:     ruleValidation(id),
			References:     ruleReferences(category), DefaultParameters: parameters,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func ruleRecommendation(id string) string {
	switch id {
	case "integrity.missing_primary_key":
		return "Confirm entity identity and downstream dependencies; assess a primary key only after data validation."
	case "integrity.fk_without_index":
		return "Review child-table query plans and write costs before considering an index whose leading keys match the FK."
	case "integrity.constraint_unvalidated":
		return "Investigate violating rows and plan controlled VALIDATE CONSTRAINT after remediation."
	case "integrity.fk_type_mismatch":
		return "Review casts and relationship semantics; do not infer the intended business type."
	case "integrity.orphan_sequence", "integrity.sequence_default_mismatch":
		return "Check application calls and defaults before assigning ownership or retiring the sequence."
	case "index.overlap", "index.prefix_overlap":
		return "Compare predicates, INCLUDE columns, uniqueness, usage and plans; never drop automatically."
	case "index.invalid":
		return "Investigate failed concurrent builds and plan a safe rebuild if needed."
	case "index.write_burden":
		return "Measure write latency and index usage before changing index inventory."
	case "index.investigate_missing":
		return "Inspect representative EXPLAIN plans; this fingerprint does not identify index columns."
	case "maintenance.stale_analyze", "maintenance.dead_tuple_pressure":
		return "Check autovacuum settings and table activity; confirm estimates with a controlled diagnostic."
	case "model.wide_table", "model.repeated_columns", "model.duplicate_entity", "model.implicit_relationship", "model.jsonb_critical":
		return "Treat as a modelling hypothesis; validate access patterns and domain boundaries with owners."
	case "model.type_review":
		return "Discuss type semantics with domain owners before proposing a migration."
	case "model.naming_inconsistent":
		return "Review naming conventions; avoid renaming without compatibility planning."
	case "model.undocumented_critical":
		return "Add catalog documentation for ownership, purpose and retention after owner review."
	case "performance.workload_scan", "performance.workload_write", "performance.workload_cost":
		return "Review the representative fingerprint and plan in an authorized environment; no raw SQL is stored."
	default:
		return "Investigate the evidence and assess business intent before changing schema or configuration."
	}
}

func ruleValidation(id string) string {
	switch {
	case strings.HasPrefix(id, "integrity."):
		return "Confirm catalog metadata and sample affected relationships in a controlled environment."
	case strings.HasPrefix(id, "index."), strings.HasPrefix(id, "performance."):
		return "Compare EXPLAIN plans and metrics over a sufficient observation window."
	case strings.HasPrefix(id, "model."):
		return "Review with domain owners; the heuristic alone is not proof of a design defect."
	default:
		return "Confirm with catalog metadata, representative workload and a controlled non-production test."
	}
}

func ruleReferences(category string) []string {
	switch category {
	case "integrity", "model":
		return []string{"https://www.postgresql.org/docs/current/ddl-constraints.html"}
	case "index":
		return []string{"https://www.postgresql.org/docs/current/indexes.html"}
	case "maintenance", "vacuum":
		return []string{"https://www.postgresql.org/docs/current/routine-vacuuming.html"}
	case "performance":
		return []string{"https://www.postgresql.org/docs/current/pgstatstatements.html"}
	case "security":
		return []string{"https://www.postgresql.org/docs/current/user-manag.html"}
	default:
		return []string{"https://www.postgresql.org/docs/current/monitoring-stats.html"}
	}
}

func definitionByID(id string) (RuleDefinition, bool) {
	for _, rule := range Catalog() {
		if rule.ID == id {
			return rule, true
		}
	}
	return RuleDefinition{}, false
}

func effectivePolicy(f SnapshotFacts, schema, id string) (bool, map[string]any) {
	def, ok := definitionByID(id)
	if !ok {
		return true, map[string]any{}
	}
	params := make(map[string]any, len(def.DefaultParameters))
	for k, v := range def.DefaultParameters {
		params[k] = v
	}
	enabled := true
	for _, scope := range []string{"", schema} {
		for _, p := range f.RulePolicies {
			if p.RuleID != id || p.SchemaName != scope || p.EnvironmentID != f.EnvironmentID {
				continue
			}
			enabled = p.Enabled
			for k, v := range p.Parameters {
				if _, allowed := def.DefaultParameters[k]; allowed {
					params[k] = v
				}
			}
		}
	}
	return enabled, params
}

func threshold(f SnapshotFacts, schema, id, name string, fallback float64) float64 {
	_, params := effectivePolicy(f, schema, id)
	if v, ok := params[name].(float64); ok && v > 0 {
		return v
	}
	return fallback
}

func ruleFlag(f SnapshotFacts, schema, id, name string, fallback bool) bool {
	_, params := effectivePolicy(f, schema, id)
	if value, ok := params[name].(bool); ok {
		return value
	}
	return fallback
}

func namingConvention(f SnapshotFacts, schema string) string {
	_, params := effectivePolicy(f, schema, "model.naming_inconsistent")
	if value, ok := params["convention"].(string); ok {
		return value
	}
	return "snake_case"
}

// EnrichFindings applies effective policies and freezes rule metadata on each
// finding, so later catalog updates cannot silently mutate historical rows.
func EnrichFindings(f SnapshotFacts, items []Finding) []Finding {
	out := make([]Finding, 0, len(items))
	for _, item := range items {
		def, ok := definitionByID(item.FindingType)
		if !ok {
			category := strings.SplitN(item.FindingType, ".", 2)[0]
			def = RuleDefinition{ID: item.FindingType, Version: "1.0.0", Category: category, Confidence: 0.5,
				Impact: "Requires human review", Risk: "uncatalogued", Recommendation: "Investigate evidence before making changes.",
				Validation: "Confirm in a controlled environment.", References: ruleReferences(category), DefaultParameters: map[string]any{}}
		}
		enabled, params := effectivePolicy(f, item.SchemaName, def.ID)
		if !enabled {
			continue
		}
		item.RuleID = def.ID
		item.RuleVersion = def.Version
		item.Category = def.Category
		item.Confidence = def.Confidence
		item.Impact = def.Impact
		item.Risk = def.Risk
		item.Recommendation = def.Recommendation
		item.Validation = def.Validation
		item.References = append([]string(nil), def.References...)
		item.RuleParameters = params
		fingerprint, _ := json.Marshal(params)
		item.DedupKey = DedupKey(item.FindingType+"@"+def.Version, item.ObjectKey, item.Title+string(fingerprint))
		out = append(out, item)
	}
	return out
}

type EffectiveRule struct {
	RuleDefinition
	Enabled             bool           `json:"enabled"`
	EffectiveParameters map[string]any `json:"effective_parameters"`
}

func EffectiveCatalog(f SnapshotFacts, schema string) []EffectiveRule {
	definitions := Catalog()
	out := make([]EffectiveRule, 0, len(definitions))
	for _, d := range definitions {
		enabled, parameters := effectivePolicy(f, schema, d.ID)
		out = append(out, EffectiveRule{RuleDefinition: d, Enabled: enabled, EffectiveParameters: parameters})
	}
	return out
}
