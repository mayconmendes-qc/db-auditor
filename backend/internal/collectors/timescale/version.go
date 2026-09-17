package timescale

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// CollectVersion returns TimescaleDB extension version facts for the current database.
// If the extension is not installed, it returns (nil, nil).
func CollectVersion(ctx context.Context, conn *pgx.Conn) (*VersionFacts, error) {
	rows, err := conn.Query(ctx, versionSQL)
	if err != nil {
		return nil, fmt.Errorf("timescale version collector: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("timescale version collector rows: %w", err)
		}
		return nil, nil
	}

	var f VersionFacts
	if err := rows.Scan(
		&f.DatabaseName,
		&f.ExtensionName,
		&f.ExtensionVersion,
		&f.SchemaName,
	); err != nil {
		return nil, fmt.Errorf("timescale version collector scan: %w", err)
	}
	major, minor, patch, err := ParseVersion(f.ExtensionVersion)
	if err != nil {
		f.Compatible = false
		f.CompatibilityNote = err.Error()
	} else {
		f.Major, f.Minor, f.Patch = major, minor, patch
		f.Compatible, f.CompatibilityNote = EvaluateCompatibility(major, f.ExtensionVersion)
	}
	return &f, nil
}
