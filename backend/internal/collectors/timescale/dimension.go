package timescale

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/mayconmendes-qc/timescale-auditor/internal/config"
)

// CollectDimensions lists hypertable dimensions and applies schema scope.
func CollectDimensions(ctx context.Context, conn *pgx.Conn, scope config.Scope) ([]DimensionFacts, error) {
	rows, err := conn.Query(ctx, dimensionsSQL)
	if err != nil {
		return nil, fmt.Errorf("dimension collector: %w", err)
	}
	defer rows.Close()

	out := make([]DimensionFacts, 0)
	for rows.Next() {
		var f DimensionFacts
		var timeInterval, integerInterval, integerNowFunc, partitioningFunc *string
		var numSlices *int
		if err := rows.Scan(
			&f.DatabaseName,
			&f.SchemaName,
			&f.HypertableName,
			&f.DimensionNumber,
			&f.ColumnName,
			&f.ColumnType,
			&f.DimensionType,
			&timeInterval,
			&integerInterval,
			&integerNowFunc,
			&numSlices,
			&partitioningFunc,
		); err != nil {
			return nil, fmt.Errorf("dimension collector scan: %w", err)
		}
		f.TimeInterval = timeInterval
		f.IntegerInterval = integerInterval
		f.IntegerNowFunc = integerNowFunc
		f.NumSlices = numSlices
		f.PartitioningFunc = partitioningFunc
		if !scope.AllowsSchema(f.SchemaName) {
			continue
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("dimension collector rows: %w", err)
	}
	return out, nil
}
