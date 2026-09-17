package repository

import (
	"context"
	"time"
)

type HypertableSnapshotRow struct {
	ID                 string    `json:"id"`
	AuditRunID         string    `json:"audit_run_id"`
	EnvironmentID      string    `json:"environment_id"`
	DatabaseName       string    `json:"database_name"`
	SchemaName         string    `json:"schema_name"`
	HypertableName     string    `json:"hypertable_name"`
	OwnerName          *string   `json:"owner_name"`
	NumDimensions      int       `json:"num_dimensions"`
	NumChunks          int       `json:"num_chunks"`
	CompressionEnabled bool      `json:"compression_enabled"`
	IsDistributed      bool      `json:"is_distributed"`
	TotalSizeBytes     int64     `json:"total_size_bytes"`
	DataSizeBytes      int64     `json:"data_size_bytes"`
	IndexSizeBytes     int64     `json:"index_size_bytes"`
	CollectedAt        time.Time `json:"collected_at"`
}

type DimensionSnapshotRow struct {
	ID              string    `json:"id"`
	DatabaseName    string    `json:"database_name"`
	SchemaName      string    `json:"schema_name"`
	HypertableName  string    `json:"hypertable_name"`
	DimensionNumber int       `json:"dimension_number"`
	ColumnName      string    `json:"column_name"`
	ColumnType      *string   `json:"column_type"`
	DimensionType   *string   `json:"dimension_type"`
	TimeInterval    *string   `json:"time_interval"`
	CollectedAt     time.Time `json:"collected_at"`
}

type ChunkSnapshotRow struct {
	ID            string     `json:"id"`
	DatabaseName  string     `json:"database_name"`
	SchemaName    string     `json:"schema_name"`
	HypertableName string    `json:"hypertable_name"`
	ChunkSchema   string     `json:"chunk_schema"`
	ChunkName     string     `json:"chunk_name"`
	RangeStart    *time.Time `json:"range_start"`
	RangeEnd      *time.Time `json:"range_end"`
	IsCompressed  bool       `json:"is_compressed"`
	TotalSizeBytes int64     `json:"total_size_bytes"`
	CollectedAt   time.Time  `json:"collected_at"`
}

type CAGGSnapshotRow struct {
	ID                        string    `json:"id"`
	DatabaseName              string    `json:"database_name"`
	SchemaName                string    `json:"schema_name"`
	ViewName                  string    `json:"view_name"`
	OwnerName                 *string   `json:"owner_name"`
	MaterializationSchema     *string   `json:"materialization_schema"`
	MaterializationHypertable *string   `json:"materialization_hypertable"`
	MaterializedOnly          bool      `json:"materialized_only"`
	CompressionEnabled        bool      `json:"compression_enabled"`
	CollectedAt               time.Time `json:"collected_at"`
}

type JobSnapshotRow struct {
	ID               string     `json:"id"`
	DatabaseName     string     `json:"database_name"`
	JobID            int64      `json:"job_id"`
	ApplicationName  *string    `json:"application_name"`
	ProcName         *string    `json:"proc_name"`
	Scheduled        bool       `json:"scheduled"`
	ScheduleInterval *string    `json:"schedule_interval"`
	NextStart        *time.Time `json:"next_start"`
	HypertableSchema *string    `json:"hypertable_schema"`
	HypertableName   *string    `json:"hypertable_name"`
	CollectedAt      time.Time  `json:"collected_at"`
}

type PolicySnapshotRow struct {
	ID               string     `json:"id"`
	DatabaseName     string     `json:"database_name"`
	JobID            int64      `json:"job_id"`
	PolicyType       string     `json:"policy_type"`
	ProcName         *string    `json:"proc_name"`
	HypertableSchema *string    `json:"hypertable_schema"`
	HypertableName   *string    `json:"hypertable_name"`
	ScheduleInterval *string    `json:"schedule_interval"`
	Scheduled        bool       `json:"scheduled"`
	NextStart        *time.Time `json:"next_start"`
	CollectedAt      time.Time  `json:"collected_at"`
}

func (s *Store) ListHypertableSnapshots(ctx context.Context, environmentID string) ([]HypertableSnapshotRow, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id::text, audit_run_id::text, environment_id::text, database_name, schema_name, hypertable_name,
  owner_name, num_dimensions, num_chunks, compression_enabled, is_distributed,
  total_size_bytes, data_size_bytes, index_size_bytes, collected_at
FROM hypertable_snapshot
WHERE environment_id = $1::uuid
ORDER BY collected_at DESC, database_name, schema_name, hypertable_name
`, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]HypertableSnapshotRow, 0)
	for rows.Next() {
		var r HypertableSnapshotRow
		if err := rows.Scan(
			&r.ID, &r.AuditRunID, &r.EnvironmentID, &r.DatabaseName, &r.SchemaName, &r.HypertableName,
			&r.OwnerName, &r.NumDimensions, &r.NumChunks, &r.CompressionEnabled, &r.IsDistributed,
			&r.TotalSizeBytes, &r.DataSizeBytes, &r.IndexSizeBytes, &r.CollectedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) ListDimensionSnapshots(ctx context.Context, environmentID string) ([]DimensionSnapshotRow, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id::text, database_name, schema_name, hypertable_name, dimension_number,
  column_name, column_type, dimension_type, time_interval, collected_at
FROM dimension_snapshot
WHERE environment_id = $1::uuid
ORDER BY collected_at DESC, database_name, schema_name, hypertable_name, dimension_number
`, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]DimensionSnapshotRow, 0)
	for rows.Next() {
		var r DimensionSnapshotRow
		if err := rows.Scan(
			&r.ID, &r.DatabaseName, &r.SchemaName, &r.HypertableName, &r.DimensionNumber,
			&r.ColumnName, &r.ColumnType, &r.DimensionType, &r.TimeInterval, &r.CollectedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) ListChunkSnapshots(ctx context.Context, environmentID string) ([]ChunkSnapshotRow, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id::text, database_name, schema_name, hypertable_name, chunk_schema, chunk_name,
  range_start, range_end, is_compressed, total_size_bytes, collected_at
FROM chunk_snapshot
WHERE environment_id = $1::uuid
ORDER BY collected_at DESC, database_name, schema_name, hypertable_name, chunk_name
`, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ChunkSnapshotRow, 0)
	for rows.Next() {
		var r ChunkSnapshotRow
		if err := rows.Scan(
			&r.ID, &r.DatabaseName, &r.SchemaName, &r.HypertableName, &r.ChunkSchema, &r.ChunkName,
			&r.RangeStart, &r.RangeEnd, &r.IsCompressed, &r.TotalSizeBytes, &r.CollectedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) ListCAGGSnapshots(ctx context.Context, environmentID string) ([]CAGGSnapshotRow, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id::text, database_name, schema_name, view_name, owner_name,
  materialization_schema, materialization_hypertable, materialized_only, compression_enabled, collected_at
FROM continuous_aggregate_snapshot
WHERE environment_id = $1::uuid
ORDER BY collected_at DESC, database_name, schema_name, view_name
`, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]CAGGSnapshotRow, 0)
	for rows.Next() {
		var r CAGGSnapshotRow
		if err := rows.Scan(
			&r.ID, &r.DatabaseName, &r.SchemaName, &r.ViewName, &r.OwnerName,
			&r.MaterializationSchema, &r.MaterializationHypertable, &r.MaterializedOnly, &r.CompressionEnabled, &r.CollectedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) ListJobSnapshots(ctx context.Context, environmentID string) ([]JobSnapshotRow, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id::text, database_name, job_id, application_name, proc_name, scheduled,
  schedule_interval, next_start, hypertable_schema, hypertable_name, collected_at
FROM job_snapshot
WHERE environment_id = $1::uuid
ORDER BY collected_at DESC, job_id
`, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]JobSnapshotRow, 0)
	for rows.Next() {
		var r JobSnapshotRow
		if err := rows.Scan(
			&r.ID, &r.DatabaseName, &r.JobID, &r.ApplicationName, &r.ProcName, &r.Scheduled,
			&r.ScheduleInterval, &r.NextStart, &r.HypertableSchema, &r.HypertableName, &r.CollectedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) ListPolicySnapshots(ctx context.Context, environmentID string) ([]PolicySnapshotRow, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id::text, database_name, job_id, policy_type, proc_name, hypertable_schema, hypertable_name,
  schedule_interval, scheduled, next_start, collected_at
FROM policy_snapshot
WHERE environment_id = $1::uuid
ORDER BY collected_at DESC, policy_type, job_id
`, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PolicySnapshotRow, 0)
	for rows.Next() {
		var r PolicySnapshotRow
		if err := rows.Scan(
			&r.ID, &r.DatabaseName, &r.JobID, &r.PolicyType, &r.ProcName, &r.HypertableSchema, &r.HypertableName,
			&r.ScheduleInterval, &r.Scheduled, &r.NextStart, &r.CollectedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
