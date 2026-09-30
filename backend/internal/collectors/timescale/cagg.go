package timescale

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/mayconmendes-qc/db-auditor/internal/config"
)

// ContinuousAggregateFacts is one continuous aggregate row.
type ContinuousAggregateFacts struct {
	DatabaseName              string `json:"database_name"`
	SchemaName                string `json:"schema_name"`
	ViewName                  string `json:"view_name"`
	Owner                     string `json:"owner_name"`
	MaterializationSchema     string `json:"materialization_schema"`
	MaterializationHypertable string `json:"materialization_hypertable"`
	MaterializedOnly          bool   `json:"materialized_only"`
	CompressionEnabled        bool   `json:"compression_enabled"`
	Finalized                 *bool  `json:"finalized,omitempty"`
	ViewDefinition            string `json:"view_definition"`
}

// CollectContinuousAggregates lists CAGGs and applies schema scope on view schema.
// Returns an empty slice when timescaledb is not installed in this database.
func CollectContinuousAggregates(ctx context.Context, conn *pgx.Conn, scope config.Scope) ([]ContinuousAggregateFacts, error) {
	ok, err := ensureExtension(ctx, conn, "cagg collector")
	if err != nil {
		return nil, err
	}
	if !ok {
		return []ContinuousAggregateFacts{}, nil
	}

	rows, err := conn.Query(ctx, continuousAggregatesSQL)
	if err != nil {
		return nil, fmt.Errorf("cagg collector: %w", err)
	}
	defer rows.Close()

	out := make([]ContinuousAggregateFacts, 0)
	for rows.Next() {
		var f ContinuousAggregateFacts
		var finalized *bool
		if err := rows.Scan(
			&f.DatabaseName,
			&f.SchemaName,
			&f.ViewName,
			&f.Owner,
			&f.MaterializationSchema,
			&f.MaterializationHypertable,
			&f.MaterializedOnly,
			&f.CompressionEnabled,
			&finalized,
			&f.ViewDefinition,
		); err != nil {
			return nil, fmt.Errorf("cagg collector scan: %w", err)
		}
		f.Finalized = finalized
		if !scope.AllowsSchema(f.SchemaName) {
			continue
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("cagg collector rows: %w", err)
	}
	return out, nil
}
