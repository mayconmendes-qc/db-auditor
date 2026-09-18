package timescale

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ExtensionInstalled reports whether the timescaledb extension exists in the
// current database. Timescale catalogs (timescaledb_information.*) are
// database-local; connecting to "postgres" often yields false even when
// application databases on the same cluster have the extension.
func ExtensionInstalled(ctx context.Context, conn *pgx.Conn) (bool, error) {
	var ok bool
	err := conn.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1 FROM pg_extension WHERE extname = 'timescaledb'
)`).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("check timescaledb extension: %w", err)
	}
	return ok, nil
}

// ensureExtension returns (false, nil) when Timescale is not installed so
// collectors can no-op with success; (true, nil) when views may be queried.
func ensureExtension(ctx context.Context, conn *pgx.Conn, collector string) (bool, error) {
	ok, err := ExtensionInstalled(ctx, conn)
	if err != nil {
		return false, fmt.Errorf("%s: %w", collector, err)
	}
	return ok, nil
}
