package repository

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type InventoryFilter struct {
	EnvironmentID string
	Database      string
	Schema        string
	Table         string
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

func inventoryWhereFor(table string, f InventoryFilter, startArg int) (string, []any, int) {
	conds := []string{fmt.Sprintf("environment_id = $%d::uuid", startArg)}
	args := []any{f.EnvironmentID}
	n := startArg + 1
	conds = append(conds, fmt.Sprintf(`audit_run_id = (
  SELECT audit_run_id FROM %s
  WHERE environment_id = $%d::uuid
  ORDER BY collected_at DESC
  LIMIT 1
)`, table, startArg))
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
	return strings.Join(conds, " AND "), args, n
}

func (s *Store) ListTableSnapshots(ctx context.Context, f InventoryFilter) ([]TableSnapshotRow, int, error) {
	where, args, next := inventoryWhereFor("table_snapshot", f, 1)
	if f.Q != "" {
		where += fmt.Sprintf(" AND (table_name ILIKE $%d OR schema_name ILIKE $%d OR database_name ILIKE $%d)", next, next, next)
		args = append(args, "%"+f.Q+"%")
		next++
	}
	var total int
	if err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM table_snapshot WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit, offset := f.Limit, f.Offset
	if limit <= 0 {
		limit = 50
	}
	query := fmt.Sprintf(`
SELECT id::text, audit_run_id::text, environment_id::text, database_name, schema_name, table_name,
  owner_name, COALESCE(relkind,''), total_size_bytes, data_size_bytes, index_size_bytes,
  COALESCE(row_estimate,0), COALESCE(column_count,0), COALESCE(has_primary_key,false), collected_at
FROM table_snapshot
WHERE %s
ORDER BY database_name, schema_name, table_name
LIMIT $%d OFFSET $%d
`, where, next, next+1)
	args = append(args, limit, offset)
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]TableSnapshotRow, 0)
	for rows.Next() {
		var r TableSnapshotRow
		if err := rows.Scan(
			&r.ID, &r.AuditRunID, &r.EnvironmentID, &r.DatabaseName, &r.SchemaName, &r.TableName,
			&r.OwnerName, &r.Relkind, &r.TotalSizeBytes, &r.DataSizeBytes, &r.IndexSizeBytes,
			&r.RowEstimate, &r.ColumnCount, &r.HasPrimaryKey, &r.CollectedAt,
		); err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

func (s *Store) ListColumnSnapshots(ctx context.Context, f InventoryFilter) ([]ColumnSnapshotRow, int, error) {
	where, args, next := inventoryWhereFor("column_snapshot", f, 1)
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
  identity_generation, collation_name, collected_at
FROM column_snapshot
WHERE %s
ORDER BY database_name, schema_name, table_name, ordinal_position
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
			&r.IdentityGeneration, &r.CollationName, &r.CollectedAt,
		); err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

func (s *Store) ListIndexSnapshots(ctx context.Context, f InventoryFilter) ([]IndexSnapshotRow, int, error) {
	where, args, next := inventoryWhereFor("index_snapshot", f, 1)
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
ORDER BY database_name, schema_name, index_name
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
	where, args, next := inventoryWhereFor("view_snapshot", f, 1)
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
ORDER BY database_name, schema_name, view_name
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
	where, args, next := inventoryWhereFor("function_snapshot", f, 1)
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
ORDER BY database_name, schema_name, function_name
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
