package repository

import "context"

func (s *Store) ListHypertableSnapshotsPage(ctx context.Context, f InventoryFilter) ([]HypertableSnapshotRow, int, error) {
	runID, err := s.ResolveAuditRunID(ctx, f.EnvironmentID, f.AuditRunID)
	if err != nil {
		return nil, 0, err
	}
	where := `environment_id=$1::uuid AND audit_run_id=$2::uuid
AND ($3='' OR database_name=$3) AND ($4='' OR schema_name=$4)
AND ($5='' OR hypertable_name ILIKE $5 OR schema_name ILIKE $5)`
	args := []any{f.EnvironmentID, runID, f.Database, f.Schema, likePattern(f.Q)}
	var total int
	if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM hypertable_snapshot WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit, offset := pageBounds(f.Limit, f.Offset, 20)
	rows, err := s.pool.Query(ctx, `SELECT id::text,audit_run_id::text,environment_id::text,database_name,schema_name,hypertable_name,
owner_name,num_dimensions,num_chunks,compression_enabled,is_distributed,total_size_bytes,data_size_bytes,index_size_bytes,collected_at
FROM hypertable_snapshot WHERE `+where+`
ORDER BY database_name,schema_name,hypertable_name,id LIMIT $6 OFFSET $7`, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []HypertableSnapshotRow{}
	for rows.Next() {
		var item HypertableSnapshotRow
		if err := rows.Scan(&item.ID, &item.AuditRunID, &item.EnvironmentID, &item.DatabaseName, &item.SchemaName, &item.HypertableName,
			&item.OwnerName, &item.NumDimensions, &item.NumChunks, &item.CompressionEnabled, &item.IsDistributed,
			&item.TotalSizeBytes, &item.DataSizeBytes, &item.IndexSizeBytes, &item.CollectedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (s *Store) ListCAGGSnapshotsPage(ctx context.Context, f InventoryFilter) ([]CAGGSnapshotRow, int, error) {
	runID, err := s.ResolveAuditRunID(ctx, f.EnvironmentID, f.AuditRunID)
	if err != nil {
		return nil, 0, err
	}
	where := `environment_id=$1::uuid AND audit_run_id=$2::uuid
AND ($3='' OR database_name=$3) AND ($4='' OR schema_name=$4)
AND ($5='' OR view_name ILIKE $5 OR schema_name ILIKE $5)`
	args := []any{f.EnvironmentID, runID, f.Database, f.Schema, likePattern(f.Q)}
	var total int
	if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM continuous_aggregate_snapshot WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit, offset := pageBounds(f.Limit, f.Offset, 20)
	rows, err := s.pool.Query(ctx, `SELECT id::text,database_name,schema_name,view_name,owner_name,
materialization_schema,materialization_hypertable,materialized_only,compression_enabled,collected_at
FROM continuous_aggregate_snapshot WHERE `+where+`
ORDER BY database_name,schema_name,view_name,id LIMIT $6 OFFSET $7`, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []CAGGSnapshotRow{}
	for rows.Next() {
		var item CAGGSnapshotRow
		if err := rows.Scan(&item.ID, &item.DatabaseName, &item.SchemaName, &item.ViewName, &item.OwnerName,
			&item.MaterializationSchema, &item.MaterializationHypertable, &item.MaterializedOnly, &item.CompressionEnabled, &item.CollectedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}
