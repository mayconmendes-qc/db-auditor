package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// CollectExtensions lists installed extensions in the current database.
// Schema scope is not applied: extensions are database-level objects.
func CollectExtensions(ctx context.Context, conn *pgx.Conn) ([]ExtensionFacts, error) {
	rows, err := conn.Query(ctx, extensionsSQL)
	if err != nil {
		return nil, fmt.Errorf("extension collector: %w", err)
	}
	defer rows.Close()

	out := make([]ExtensionFacts, 0)
	for rows.Next() {
		var f ExtensionFacts
		if err := rows.Scan(
			&f.DatabaseName,
			&f.ExtensionName,
			&f.ExtensionVersion,
			&f.SchemaName,
			&f.IsRelocatable,
		); err != nil {
			return nil, fmt.Errorf("extension collector scan: %w", err)
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("extension collector rows: %w", err)
	}
	return out, nil
}
