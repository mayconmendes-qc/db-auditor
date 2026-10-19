package postgres

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The checked-in SQL is the reviewable contract for the SQL run by Go.
func TestReviewedSQLMatchesExecutedSQL(t *testing.T) {
	queries := map[string]string{
		"server": serverSQL, "databases": databasesSQL, "schemas": schemasSQL,
		"tables": tablesSQL, "columns": columnsSQL, "indexes": indexesSQL,
		"constraints": constraintsSQL, "views": viewsSQL, "functions": functionsSQL,
		"extensions": extensionsSQL, "effective_grants": effectiveGrantsSQL,
		"dependencies": objectDependenciesSQL,
		"grant_origins": grantOriginsSQL, "column_grants": columnGrantsSQL,
	}
	for name, executed := range queries {
		t.Run(name, func(t *testing.T) {
			body, err := os.ReadFile(filepath.Join("../../../sql/collectors/postgres", name+".sql"))
			if err != nil {
				t.Fatal(err)
			}
			reviewed := string(body)
			if i := strings.IndexByte(reviewed, '\n'); i >= 0 && strings.HasPrefix(reviewed, "--") {
				reviewed = reviewed[i+1:]
			}
			if strings.TrimSpace(reviewed) != strings.TrimSpace(executed) {
				t.Fatalf("%s.sql diverges from runtime collector SQL", name)
			}
		})
	}
}
