package analyzer

import (
	"context"
	"fmt"
	"strings"

	"github.com/osmendes/db-auditor/internal/schemadiff"
)

// SchemaChangeFact is a persisted timeline event loaded for analysis/reprocess.
type SchemaChangeFact struct {
	ObjectType         string
	ObjectKey          string
	ChangeKind         string
	PriorFingerprint   string
	CurrentFingerprint string
	ChangedFields      []string
	Correlation        string
	Confidence         string
	IsSecurityDefiner  bool
}

// SchemaDriftAnalyzer emits drift.out_of_pipeline from timeline facts already
// recorded at ingestion. Environment divergence is handled in the repository
// recorder (needs cross-env reads); this analyzer covers reprocess paths.
type SchemaDriftAnalyzer struct{}

func (SchemaDriftAnalyzer) Name() string { return "schema_drift" }

func (SchemaDriftAnalyzer) Analyze(_ context.Context, facts SnapshotFacts) ([]Finding, error) {
	if len(facts.SchemaChanges) == 0 {
		return nil, nil
	}
	isProd := facts.IsProduction
	var out []Finding
	for _, ev := range facts.SchemaChanges {
		if ev.Correlation != "none" && ev.Correlation != "" {
			continue
		}
		sev := schemadiff.OutOfPipelineSeverity(ev.ObjectType, ev.ChangeKind, ev.IsSecurityDefiner)
		if !isProd {
			sev = schemadiff.DowngradeSeverity(sev)
		}
		title := fmt.Sprintf("Mudança de schema fora do pipeline: %s %s", ev.ChangeKind, ev.ObjectKey)
		out = append(out, Finding{
			EnvironmentID:  facts.EnvironmentID,
			AuditRunID:     facts.AuditRunID,
			FindingType:    "drift.out_of_pipeline",
			RuleID:         "drift.out_of_pipeline",
			RuleVersion:    "1",
			Category:       "structure",
			Confidence:     0.8,
			Severity:       Severity(sev),
			Status:         StatusOpen,
			Title:          title,
			Summary:        "Objeto alterado entre coletas completas sem evidência de pipeline.",
			ObjectType:     ev.ObjectType,
			ObjectKey:      ev.ObjectKey,
			Recommendation: "Confirme se a mudança foi intencional e registre a implantação.",
			Validation:     "Consulte a linha do tempo de schema do ambiente.",
			Evidence: map[string]any{
				"change_kind":         ev.ChangeKind,
				"prior_fingerprint":   ev.PriorFingerprint,
				"current_fingerprint": ev.CurrentFingerprint,
				"changed_fields":      ev.ChangedFields,
				"correlation":         ev.Correlation,
			},
			DedupKey: DedupKey("drift.out_of_pipeline", ev.ObjectKey, ev.ChangeKind+"@"+facts.AuditRunID),
		})
	}
	return out, nil
}

// EnvironmentDivergenceFinding builds one persistent cross-env finding.
func EnvironmentDivergenceFinding(env, run, objectKey, peerEnv string, isProd bool) Finding {
	sev := "medium"
	if !isProd {
		sev = schemadiff.DowngradeSeverity(sev)
	}
	return Finding{
		EnvironmentID:  env,
		AuditRunID:     run,
		FindingType:    "drift.environment_divergence",
		RuleID:         "drift.environment_divergence",
		RuleVersion:    "1",
		Category:       "structure",
		Confidence:     0.7,
		Severity:       Severity(sev),
		Status:         StatusOpen,
		Title:          "Divergência persistente entre ambientes: " + objectKey,
		Summary:        "O objeto diverge do ambiente pareado em duas coletas completas de cada lado.",
		ObjectKey:      objectKey,
		Recommendation: "Alinhe os ambientes ou confirme que a diferença é intencional.",
		Validation:     "Compare as execuções na tela Desvio de schema.",
		Evidence:       map[string]any{"peer_environment_id": peerEnv},
		DedupKey:       DedupKey("drift.environment_divergence", objectKey, peerEnv),
	}
}

// SanitizeDriftEvidence ensures no raw SQL slipped into evidence maps.
func SanitizeDriftEvidence(ev map[string]any) map[string]any {
	if ev == nil {
		return nil
	}
	out := make(map[string]any, len(ev))
	for k, v := range ev {
		s, ok := v.(string)
		if ok && looksLikeSQL(s) {
			continue
		}
		out[k] = v
	}
	return out
}

func looksLikeSQL(s string) bool {
	u := strings.ToUpper(strings.TrimSpace(s))
	return strings.HasPrefix(u, "SELECT ") || strings.HasPrefix(u, "CREATE ") ||
		strings.HasPrefix(u, "ALTER ") || strings.Contains(u, " FROM ")
}
