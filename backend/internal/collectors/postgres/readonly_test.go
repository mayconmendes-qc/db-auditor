package postgres

import (
	"strings"
	"testing"
)

// T-296: collectors must remain read-only (SELECT / WITH only).
func TestCollectorSQLIsReadOnly(t *testing.T) {
	t.Parallel()
	queries := []string{
		serverSQL, databasesSQL, schemasSQL, tablesSQL, columnsSQL,
		indexesSQL, constraintsSQL, viewsSQL, functionsSQL, extensionsSQL,
		sequencesSQL, triggersSQL, policiesSQL,
		columnStatsSQL, workloadAvailableSQL, workloadSQL,
	}
	forbidden := []string{
		"insert ", "update ", "delete ", "drop ", "alter ", "truncate ",
		"create ", "grant ", "revoke ", "vacuum ", "reindex ",
	}
	for i, q := range queries {
		lower := strings.ToLower(q)
		trimmed := strings.TrimSpace(lower)
		if !strings.HasPrefix(trimmed, "select") && !strings.HasPrefix(trimmed, "with") {
			t.Fatalf("query %d does not start with SELECT/WITH", i)
		}
		for _, bad := range forbidden {
			if strings.Contains(lower, bad) {
				t.Fatalf("query %d contains forbidden keyword %q", i, strings.TrimSpace(bad))
			}
		}
	}
}
