package analyzer

import (
	"testing"

	"github.com/osmendes/db-auditor/internal/pii"
)

func TestBroadReadAccessSevenLogins(t *testing.T) {
	accounts := []string{"a1", "a2", "a3", "a4", "a5", "a6", "a7"}
	findings := BuildPIIFindings("env", "run", []PIIColumnFact{{
		Database: "app", Schema: "public", Table: "pessoa", Column: "cpf",
		Category: pii.CatCPF, Confidence: pii.ConfidenceAlta, Method: pii.MethodAmbos,
		ReadAccounts: 7, ReadNames: accounts, AccessLevel: "nivel_tabela",
	}})
	var broad *Finding
	for i := range findings {
		if findings[i].FindingType == "pii.broad_read_access" {
			broad = &findings[i]
			break
		}
	}
	if broad == nil {
		t.Fatal("expected pii.broad_read_access")
	}
	if broad.Severity != SeverityHigh {
		t.Fatalf("CPF should be high, got %s", broad.Severity)
	}
	names, _ := broad.Evidence["accounts"].([]string)
	if len(names) != 7 {
		t.Fatalf("accounts in evidence: %v", broad.Evidence["accounts"])
	}
}

func TestColumnDetectedMedium(t *testing.T) {
	findings := BuildPIIFindings("e", "r", []PIIColumnFact{{
		Database: "app", Schema: "public", Table: "t", Column: "email",
		Category: pii.CatEmail, Confidence: pii.ConfidenceMedia, Method: pii.MethodNome,
	}})
	if len(findings) == 0 || findings[0].FindingType != "pii.column_detected" {
		t.Fatalf("%+v", findings)
	}
}
