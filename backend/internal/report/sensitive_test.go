package report

import (
	"strings"
	"testing"
)

func TestRedactionRemovesSensitiveValuesFromPDF(t *testing.T) {
	for _, level := range []string{"identifiers", "strict"} {
		d := Document{Type: "technical", Environment: "secret-env", RequestedBy: "secret-user", DatabaseFilter: "secret-db",
			Databases: []Database{{Name: "secret-db"}}, Tables: []Table{{Database: "secret-db", Schema: "secret-schema", Name: "secret-table"}},
			Findings:      []Finding{{Type: "model.no_primary_key", Database: "secret-db", Schema: "secret-schema", Object: "secret-table", Title: "secret-title", Summary: "secret-summary", Recommendation: "secret-recommendation", Evidence: "secret-evidence"}},
			CoverageNotes: []string{"secret-note"}}
		d.RedactSensitive(level)
		lines := BuildLines(d)
		var content strings.Builder
		for _, line := range lines {
			content.WriteString(line.Text)
		}
		for _, secret := range []string{"secret-env", "secret-user", "secret-db", "secret-schema", "secret-table", "secret-title", "secret-summary", "secret-recommendation", "secret-evidence", "secret-note"} {
			if strings.Contains(content.String(), secret) {
				t.Fatalf("sensitive value %q leaked", secret)
			}
		}
		if !strings.Contains(content.String(), "Nível de redação: "+level) {
			t.Fatal("redaction level missing")
		}
	}
}
