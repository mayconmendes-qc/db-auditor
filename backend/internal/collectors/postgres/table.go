package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mayconmendes-qc/db-auditor/internal/config"
)

// CollectTables lists user tables in the current database and applies schema scope filters.
func CollectTables(ctx context.Context, conn *pgx.Conn, scope config.Scope) ([]TableFacts, error) {
	rows, err := conn.Query(ctx, tablesSQL)
	if err != nil {
		return nil, fmt.Errorf("table collector: %w", err)
	}
	defer rows.Close()

	out := make([]TableFacts, 0)
	for rows.Next() {
		var f TableFacts
		var lastVacuum, lastAutovacuum, lastAnalyze, lastAutoanalyze *time.Time
		if err := rows.Scan(
			&f.DatabaseName,
			&f.SchemaName,
			&f.TableName,
			&f.Owner,
			&f.Relkind,
			&f.DataSizeBytes,
			&f.IndexSizeBytes,
			&f.TotalSizeBytes,
			&f.RowEstimate,
			&f.NLiveTup,
			&f.NDeadTup,
			&f.NTupIns,
			&f.NTupUpd,
			&f.NTupDel,
			&f.SeqScan,
			&f.IdxScan,
			&lastVacuum,
			&lastAutovacuum,
			&lastAnalyze,
			&lastAutoanalyze,
			&f.ColumnCount,
			&f.HasPrimaryKey,
		); err != nil {
			return nil, fmt.Errorf("table collector scan: %w", err)
		}
		f.LastVacuum = lastVacuum
		f.LastAutovacuum = lastAutovacuum
		f.LastAnalyze = lastAnalyze
		f.LastAutoanalyze = lastAutoanalyze
		if f.DataSizeBytes < 0 {
			f.DataSizeBytes = 0
		}
		if f.IndexSizeBytes < 0 {
			f.IndexSizeBytes = 0
		}
		if f.TotalSizeBytes < 0 {
			f.TotalSizeBytes = 0
		}
		if !scope.AllowsSchema(f.SchemaName) {
			continue
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("table collector rows: %w", err)
	}
	return out, nil
}
