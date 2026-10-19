package analyzer

import (
	"context"
	"fmt"
	"strings"

	"github.com/osmendes/db-auditor/internal/pii"
)

const (
	piiBroadReadMinLogins = 5
	piiNoRLSMinApps       = 2
)

type PIIColumnFact struct {
	Database, Schema, Table, Column string
	Category, Confidence, Method    string
	ReadAccounts                    int
	ReadNames                       []string
	HasRLS                          bool
	AccessLevel                     string
}

type PIIAnalyzer struct{}

func (PIIAnalyzer) Name() string { return "pii" }

func (PIIAnalyzer) Analyze(_ context.Context, facts SnapshotFacts) ([]Finding, error) {
	return BuildPIIFindings(facts.EnvironmentID, facts.AuditRunID, facts.PIIColumns), nil
}

func BuildPIIFindings(envID, runID string, columns []PIIColumnFact) []Finding {
	out := make([]Finding, 0)
	noRLSEmitted := map[string]bool{}
	for _, c := range columns {
		if c.Confidence != pii.ConfidenceAlta && c.Confidence != pii.ConfidenceMedia {
			continue
		}
		key := fmt.Sprintf("%s.%s.%s.%s", c.Database, c.Schema, c.Table, c.Column)
		title := fmt.Sprintf("Dado pessoal detectado: %s (%s)", key, c.Category)
		ev := map[string]any{
			"category": c.Category, "confidence": c.Confidence, "method": c.Method,
			"read_accounts": c.ReadAccounts, "has_rls": c.HasRLS, "access_level": c.AccessLevel,
		}
		if c.AccessLevel == "nivel_tabela" {
			ev["access_note"] = "nível tabela"
		}
		out = append(out, Finding{
			EnvironmentID: envID, AuditRunID: runID,
			FindingType: "pii.column_detected", Severity: SeverityInfo, Status: StatusOpen,
			Title: title, Summary: "Coluna classificada como dado pessoal com confiança média ou alta.",
			ObjectType: "column", ObjectKey: key,
			DatabaseName: c.Database, SchemaName: c.Schema, ObjectName: c.Table + "." + c.Column,
			Evidence: ev, DedupKey: DedupKey("pii.column_detected", key, title),
			RuleID: "pii.column_detected", RuleVersion: "1", Category: "security",
		})
		if c.Confidence == pii.ConfidenceAlta && c.ReadAccounts >= piiBroadReadMinLogins {
			sev := SeverityMedium
			if pii.SensitiveIdentity(c.Category) {
				sev = SeverityHigh
			}
			broadTitle := fmt.Sprintf("Acesso amplo a dado pessoal: %s", key)
			broadEv := map[string]any{
				"category": c.Category, "confidence": c.Confidence,
				"read_accounts": c.ReadAccounts, "accounts": append([]string{}, c.ReadNames...),
				"threshold": piiBroadReadMinLogins, "access_level": c.AccessLevel,
			}
			if c.AccessLevel == "nivel_tabela" {
				broadEv["access_note"] = "nível tabela"
			}
			out = append(out, Finding{
				EnvironmentID: envID, AuditRunID: runID,
				FindingType: "pii.broad_read_access", Severity: sev, Status: StatusOpen,
				Title: broadTitle, Summary: fmt.Sprintf("Coluna pessoal legível por %d contas com LOGIN (limiar %d).", c.ReadAccounts, piiBroadReadMinLogins),
				ObjectType: "column", ObjectKey: key,
				DatabaseName: c.Database, SchemaName: c.Schema, ObjectName: c.Table + "." + c.Column,
				Evidence: broadEv, DedupKey: DedupKey("pii.broad_read_access", key, broadTitle),
				RuleID: "pii.broad_read_access", RuleVersion: "1", Category: "security",
			})
		}
		tableKey := c.Database + "." + c.Schema + "." + c.Table
		if c.Confidence == pii.ConfidenceAlta && !c.HasRLS && c.ReadAccounts >= piiNoRLSMinApps && !noRLSEmitted[tableKey] {
			noRLSEmitted[tableKey] = true
			hypTitle := fmt.Sprintf("Hipótese: dado pessoal sem RLS em %s", tableKey)
			out = append(out, Finding{
				EnvironmentID: envID, AuditRunID: runID,
				FindingType: "pii.no_rls_hypothesis", Severity: SeverityLow, Status: StatusOpen,
				Title: hypTitle, Summary: "Tabela com dado pessoal de confiança alta, sem RLS e com múltiplas contas lendo. Hipótese.",
				ObjectType: "table", ObjectKey: tableKey,
				DatabaseName: c.Database, SchemaName: c.Schema, ObjectName: c.Table,
				Evidence: map[string]any{
					"read_accounts": c.ReadAccounts, "has_rls": false, "hypothesis": true,
					"sample_column": c.Column, "category": c.Category, "access_level": c.AccessLevel,
					"access_note": map[bool]string{true: "nível tabela", false: c.AccessLevel}[c.AccessLevel == "nivel_tabela"],
				},
				DedupKey: DedupKey("pii.no_rls_hypothesis", tableKey, hypTitle),
				RuleID: "pii.no_rls_hypothesis", RuleVersion: "1", Category: "security",
			})
		}
	}
	return out
}

func HasPowerfulReader(accounts []string, powerful []string, auditor string) bool {
	set := map[string]bool{}
	for _, a := range accounts {
		set[strings.ToLower(a)] = true
	}
	for _, p := range powerful {
		if strings.EqualFold(p, auditor) {
			continue
		}
		if set[strings.ToLower(p)] {
			return true
		}
	}
	return false
}
