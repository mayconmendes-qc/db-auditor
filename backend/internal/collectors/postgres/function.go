package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/mayconmendes-qc/timescale-auditor/internal/config"
)

// CollectFunctions lists functions and procedures in the current database.
func CollectFunctions(ctx context.Context, conn *pgx.Conn, scope config.Scope) ([]FunctionFacts, error) {
	rows, err := conn.Query(ctx, functionsSQL)
	if err != nil {
		return nil, fmt.Errorf("function collector: %w", err)
	}
	defer rows.Close()

	out := make([]FunctionFacts, 0)
	for rows.Next() {
		var f FunctionFacts
		if err := rows.Scan(
			&f.DatabaseName,
			&f.SchemaName,
			&f.FunctionName,
			&f.IdentityArguments,
			&f.Owner,
			&f.LanguageName,
			&f.IsSecurityDefiner,
			&f.Volatility,
			&f.ParallelSafety,
			&f.Kind,
			&f.FunctionDefinition,
		); err != nil {
			return nil, fmt.Errorf("function collector scan: %w", err)
		}
		if !scope.AllowsSchema(f.SchemaName) {
			continue
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("function collector rows: %w", err)
	}
	return out, nil
}
