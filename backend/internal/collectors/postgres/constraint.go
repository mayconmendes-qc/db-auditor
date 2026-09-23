package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/mayconmendes-qc/db-auditor/internal/config"
)

// CollectConstraints lists table constraints in the current database.
func CollectConstraints(ctx context.Context, conn *pgx.Conn, scope config.Scope) ([]ConstraintFacts, error) {
	rows, err := conn.Query(ctx, constraintsSQL)
	if err != nil {
		return nil, fmt.Errorf("constraint collector: %w", err)
	}
	defer rows.Close()

	out := make([]ConstraintFacts, 0)
	for rows.Next() {
		var f ConstraintFacts
		if err := rows.Scan(
			&f.DatabaseName,
			&f.SchemaName,
			&f.TableName,
			&f.ConstraintName,
			&f.ConstraintType,
			&f.ConstraintDefinition,
			&f.IsValidated,
			&f.IsDeferrable,
			&f.IsDeferred,
		); err != nil {
			return nil, fmt.Errorf("constraint collector scan: %w", err)
		}
		if !scope.AllowsSchema(f.SchemaName) {
			continue
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("constraint collector rows: %w", err)
	}
	return out, nil
}
