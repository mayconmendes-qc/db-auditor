package postgres

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestOptionalPlanCostBudgetIntegration(t *testing.T) {
	dsn := os.Getenv("AUDITOR_TEST_DATABASE_URL")
	if dsn == "" || os.Getenv("AUDITOR_TEST_DISPOSABLE") != "1" {
		t.Skip("disposable integration database not explicitly enabled")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(ctx) }()
	query := `SELECT * FROM pg_catalog.pg_class ORDER BY relname`
	if err := checkPlanBudget(ctx, conn, 1e9, query); err != nil {
		t.Fatalf("safe budget rejected: %v", err)
	}
	if err := checkPlanBudget(ctx, conn, 1, query); err == nil {
		t.Fatal("over-budget query accepted")
	}
}
