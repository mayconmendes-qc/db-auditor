package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/osmendes/db-auditor/internal/database/sqlc"
)

type InventoryFilter struct {
	EnvironmentID string
	AuditRunID    string
	Database      string
	Schema        string
	Table         string
	RelationClass string
	Q             string
	Limit         int
	Offset        int
}

type TableSnapshotRow struct {
	ID             string    `json:"id"`
	AuditRunID     string    `json:"audit_run_id"`
	EnvironmentID  string    `json:"environment_id"`
	DatabaseName   string    `json:"database_name"`
	SchemaName     string    `json:"schema_name"`
	TableName      string    `json:"table_name"`
	OwnerName      *string   `json:"owner_name"`
	Relkind        string    `json:"relkind"`
	RelationClass  string    `json:"relation_class"`
	IsPartition    bool      `json:"is_partition"`
	TotalSizeBytes int64     `json:"total_size_bytes"`
	DataSizeBytes  int64     `json:"data_size_bytes"`
	IndexSizeBytes int64     `json:"index_size_bytes"`
	RowEstimate    int64     `json:"row_estimate"`
	ColumnCount    int       `json:"column_count"`
	HasPrimaryKey  bool      `json:"has_primary_key"`
	CollectedAt    time.Time `json:"collected_at"`
}

type ColumnSnapshotRow struct {
	ID                 string    `json:"id"`
	DatabaseName       string    `json:"database_name"`
	SchemaName         string    `json:"schema_name"`
	TableName          string    `json:"table_name"`
	ColumnName         string    `json:"column_name"`
	OrdinalPosition    int       `json:"ordinal_position"`
	DataType           string    `json:"data_type"`
	IsNullable         bool      `json:"is_nullable"`
	ColumnDefault      *string   `json:"column_default"`
	IsGenerated        bool      `json:"is_generated"`
	IdentityGeneration *string   `json:"identity_generation"`
	CollationName      *string   `json:"collation_name"`
	Comment            *string   `json:"comment,omitempty"`
	CollectedAt        time.Time `json:"collected_at"`
}

type IndexSnapshotRow struct {
	ID              string    `json:"id"`
	DatabaseName    string    `json:"database_name"`
	SchemaName      string    `json:"schema_name"`
	TableName       string    `json:"table_name"`
	IndexName       string    `json:"index_name"`
	IndexDefinition string    `json:"index_definition"`
	AccessMethod    *string   `json:"access_method"`
	IsUnique        bool      `json:"is_unique"`
	IsPrimary       bool      `json:"is_primary"`
	SizeBytes       int64     `json:"size_bytes"`
	IdxScan         int64     `json:"idx_scan"`
	CollectedAt     time.Time `json:"collected_at"`
}

type ViewSnapshotRow struct {
	ID           string    `json:"id"`
	DatabaseName string    `json:"database_name"`
	SchemaName   string    `json:"schema_name"`
	ViewName     string    `json:"view_name"`
	OwnerName    *string   `json:"owner_name"`
	Relkind      string    `json:"relkind"`
	SizeBytes    int64     `json:"size_bytes"`
	CollectedAt  time.Time `json:"collected_at"`
}

type FunctionSnapshotRow struct {
	ID                string    `json:"id"`
	DatabaseName      string    `json:"database_name"`
	SchemaName        string    `json:"schema_name"`
	FunctionName      string    `json:"function_name"`
	IdentityArguments string    `json:"identity_arguments"`
	OwnerName         *string   `json:"owner_name"`
	LanguageName      *string   `json:"language_name"`
	IsSecurityDefiner bool      `json:"is_security_definer"`
	Kind              *string   `json:"kind"`
	CollectedAt       time.Time `json:"collected_at"`
}

func inventoryWhereFor(f InventoryFilter, startArg int) (string, []any, int) {
	conds := []string{fmt.Sprintf("environment_id = $%d::uuid", startArg)}
	args := []any{f.EnvironmentID}
	n := startArg + 1
	conds = append(conds, fmt.Sprintf(`audit_run_id = COALESCE(
  NULLIF($%d, '')::uuid,
  (SELECT id FROM audit_run
  WHERE environment_id = $%d::uuid
    AND status IN ('success', 'partial_success')
  ORDER BY started_at DESC,id DESC
  LIMIT 1
))`, n, startArg))
	args = append(args, f.AuditRunID)
	n++
	if f.Database != "" {
		conds = append(conds, fmt.Sprintf("database_name = $%d", n))
		args = append(args, f.Database)
		n++
	}
	if f.Schema != "" {
		conds = append(conds, fmt.Sprintf("schema_name = $%d", n))
		args = append(args, f.Schema)
		n++
	}
	if f.Table != "" {
		conds = append(conds, fmt.Sprintf("table_name = $%d", n))
		args = append(args, f.Table)
		n++
	}
	if f.RelationClass != "" {
		conds = append(conds, fmt.Sprintf("relation_class = $%d", n))
		args = append(args, f.RelationClass)
		n++
	}
	return strings.Join(conds, " AND "), args, n
}

func (s *Store) ListTableSnapshots(ctx context.Context, f InventoryFilter) ([]TableSnapshotRow, int, error) {
	limit, offset := f.Limit, f.Offset
	if limit <= 0 {
		limit = 50
	}
	q := f.Q
	if q != "" {
		q = "%" + q + "%"
	}
	env, err := parseUUID(f.EnvironmentID)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountTableSnapshots(ctx, sqlc.CountTableSnapshotsParams{
		Column1: env,
		Column2: f.AuditRunID,
		Column3: f.Database,
		Column4: f.Schema,
		Column5: f.Table,
		Column6: f.RelationClass,
		Column7: q,
	})
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.q.ListTableSnapshotsFiltered(ctx, sqlc.ListTableSnapshotsFilteredParams{
		Column1: env,
		Column2: f.AuditRunID,
		Column3: f.Database,
		Column4: f.Schema,
		Column5: f.Table,
		Column6: f.RelationClass,
		Column7: q,
		Limit:   int32(limit),
		Offset:  int32(offset),
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]TableSnapshotRow, 0, len(rows))
	for _, row := range rows {
		item := TableSnapshotRow{
			ID: row.ID, AuditRunID: row.AuditRunID, EnvironmentID: row.EnvironmentID,
			DatabaseName: row.DatabaseName, SchemaName: row.SchemaName, TableName: row.TableName,
			Relkind: row.Relkind, RelationClass: row.RelationClass, IsPartition: row.IsPartition,
			TotalSizeBytes: row.TotalSizeBytes, DataSizeBytes: row.DataSizeBytes, IndexSizeBytes: row.IndexSizeBytes,
			RowEstimate: row.RowEstimate, ColumnCount: int(row.ColumnCount), HasPrimaryKey: row.HasPrimaryKey,
		}
		if row.OwnerName.Valid {
			owner := row.OwnerName.String
			item.OwnerName = &owner
		}
		if row.CollectedAt.Valid {
			item.CollectedAt = row.CollectedAt.Time
		}
		out = append(out, item)
	}
	return out, int(total), nil
}

func (s *Store) ListColumnSnapshots(ctx context.Context, f InventoryFilter) ([]ColumnSnapshotRow, int, error) {
	where, args, next := inventoryWhereFor(f, 1)
	if f.Q != "" {
		where += fmt.Sprintf(" AND (column_name ILIKE $%d OR data_type ILIKE $%d)", next, next)
		args = append(args, "%"+f.Q+"%")
		next++
	}
	var total int
	if err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM column_snapshot WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit, offset := f.Limit, f.Offset
	if limit <= 0 {
		limit = 200
	}
	query := fmt.Sprintf(`
SELECT id::text, database_name, schema_name, table_name, column_name,
  ordinal_position, data_type, is_nullable, column_default, is_generated,
  identity_generation, collation_name, column_comment, collected_at
FROM column_snapshot
WHERE %s
ORDER BY database_name, schema_name, table_name, ordinal_position,id
LIMIT $%d OFFSET $%d
`, where, next, next+1)
	args = append(args, limit, offset)
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]ColumnSnapshotRow, 0)
	for rows.Next() {
		var r ColumnSnapshotRow
		if err := rows.Scan(
			&r.ID, &r.DatabaseName, &r.SchemaName, &r.TableName, &r.ColumnName,
			&r.OrdinalPosition, &r.DataType, &r.IsNullable, &r.ColumnDefault, &r.IsGenerated,
			&r.IdentityGeneration, &r.CollationName, &r.Comment, &r.CollectedAt,
		); err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

func (s *Store) ListIndexSnapshots(ctx context.Context, f InventoryFilter) ([]IndexSnapshotRow, int, error) {
	where, args, next := inventoryWhereFor(f, 1)
	if f.Q != "" {
		where += fmt.Sprintf(" AND (index_name ILIKE $%d OR table_name ILIKE $%d OR schema_name ILIKE $%d)", next, next, next)
		args = append(args, "%"+f.Q+"%")
		next++
	}
	var total int
	if err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM index_snapshot WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit, offset := f.Limit, f.Offset
	if limit <= 0 {
		limit = 50
	}
	query := fmt.Sprintf(`
SELECT id::text, database_name, schema_name, table_name, index_name,
  COALESCE(index_definition,''), access_method, is_unique, is_primary,
  COALESCE(size_bytes,0), COALESCE(idx_scan,0), collected_at
FROM index_snapshot
WHERE %s
ORDER BY database_name, schema_name, index_name,id
LIMIT $%d OFFSET $%d
`, where, next, next+1)
	args = append(args, limit, offset)
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]IndexSnapshotRow, 0)
	for rows.Next() {
		var r IndexSnapshotRow
		if err := rows.Scan(
			&r.ID, &r.DatabaseName, &r.SchemaName, &r.TableName, &r.IndexName,
			&r.IndexDefinition, &r.AccessMethod, &r.IsUnique, &r.IsPrimary,
			&r.SizeBytes, &r.IdxScan, &r.CollectedAt,
		); err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

func (s *Store) ListViewSnapshots(ctx context.Context, f InventoryFilter) ([]ViewSnapshotRow, int, error) {
	where, args, next := inventoryWhereFor(f, 1)
	if f.Q != "" {
		where += fmt.Sprintf(" AND (view_name ILIKE $%d OR schema_name ILIKE $%d OR database_name ILIKE $%d)", next, next, next)
		args = append(args, "%"+f.Q+"%")
		next++
	}
	var total int
	if err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM view_snapshot WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit, offset := f.Limit, f.Offset
	if limit <= 0 {
		limit = 50
	}
	query := fmt.Sprintf(`
SELECT id::text, database_name, schema_name, view_name, owner_name,
  COALESCE(relkind,''), COALESCE(size_bytes,0), collected_at
FROM view_snapshot
WHERE %s
ORDER BY database_name, schema_name, view_name,id
LIMIT $%d OFFSET $%d
`, where, next, next+1)
	args = append(args, limit, offset)
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]ViewSnapshotRow, 0)
	for rows.Next() {
		var r ViewSnapshotRow
		if err := rows.Scan(
			&r.ID, &r.DatabaseName, &r.SchemaName, &r.ViewName, &r.OwnerName,
			&r.Relkind, &r.SizeBytes, &r.CollectedAt,
		); err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

func (s *Store) ListFunctionSnapshots(ctx context.Context, f InventoryFilter) ([]FunctionSnapshotRow, int, error) {
	where, args, next := inventoryWhereFor(f, 1)
	if f.Q != "" {
		where += fmt.Sprintf(" AND (function_name ILIKE $%d OR schema_name ILIKE $%d OR database_name ILIKE $%d)", next, next, next)
		args = append(args, "%"+f.Q+"%")
		next++
	}
	var total int
	if err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM function_snapshot WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit, offset := f.Limit, f.Offset
	if limit <= 0 {
		limit = 50
	}
	query := fmt.Sprintf(`
SELECT id::text, database_name, schema_name, function_name, COALESCE(identity_arguments,''),
  owner_name, language_name, COALESCE(is_security_definer,false), kind, collected_at
FROM function_snapshot
WHERE %s
ORDER BY database_name, schema_name, function_name,id
LIMIT $%d OFFSET $%d
`, where, next, next+1)
	args = append(args, limit, offset)
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]FunctionSnapshotRow, 0)
	for rows.Next() {
		var r FunctionSnapshotRow
		if err := rows.Scan(
			&r.ID, &r.DatabaseName, &r.SchemaName, &r.FunctionName, &r.IdentityArguments,
			&r.OwnerName, &r.LanguageName, &r.IsSecurityDefiner, &r.Kind, &r.CollectedAt,
		); err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}
