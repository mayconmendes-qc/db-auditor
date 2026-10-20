package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/osmendes/db-auditor/internal/collectors/postgres"
)

func (s *Store) SaveGrantOrigins(ctx context.Context, environmentID, auditRunID pgtype.UUID, origins []postgres.GrantOriginFacts, columns []postgres.ColumnGrantFacts) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, o := range origins {
		_, err = tx.Exec(ctx, `INSERT INTO grant_origin_snapshot(
  audit_run_id,environment_id,database_name,account_name,object_type,object_name,privilege,origin_kind,via_chain,inherit_path,set_path)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
ON CONFLICT(audit_run_id,database_name,account_name,object_type,object_name,privilege) DO NOTHING`,
			auditRunID, environmentID, o.DatabaseName, o.AccountName, o.ObjectType, o.ObjectName, o.Privilege,
			o.OriginKind, inheritedOrEmpty(o.ViaChain), o.InheritPath, o.SetPath)
		if err != nil {
			return err
		}
	}
	for _, c := range columns {
		_, err = tx.Exec(ctx, `INSERT INTO column_grant_snapshot(
  audit_run_id,environment_id,database_name,schema_name,table_name,column_name,account_name,privilege,origin_kind,via_chain,inherit_path,set_path)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
ON CONFLICT(audit_run_id,database_name,schema_name,table_name,column_name,account_name,privilege) DO NOTHING`,
			auditRunID, environmentID, c.DatabaseName, c.SchemaName, c.TableName, c.ColumnName, c.AccountName,
			c.Privilege, c.OriginKind, inheritedOrEmpty(c.ViaChain), c.InheritPath, c.SetPath)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// GrantOriginRow is a persisted origin used by access-review freeze and API.
type GrantOriginRow struct {
	DatabaseName string   `json:"database_name"`
	AccountName  string   `json:"account_name"`
	ObjectType   string   `json:"object_type"`
	ObjectName   string   `json:"object_name"`
	Privilege    string   `json:"privilege"`
	OriginKind   string   `json:"origin_kind"`
	ViaChain     []string `json:"via_chain"`
	OriginLabel  string   `json:"origin_label"`
	InheritPath  *bool    `json:"inherit_path,omitempty"`
	SetPath      *bool    `json:"set_path,omitempty"`
}

func (s *Store) ListGrantOrigins(ctx context.Context, env, run string) ([]GrantOriginRow, error) {
	rows, err := s.pool.Query(ctx, `SELECT database_name,account_name,object_type,object_name,privilege,origin_kind,via_chain,inherit_path,set_path
FROM grant_origin_snapshot WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid
ORDER BY account_name,object_type,object_name,privilege`, env, run)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]GrantOriginRow, 0)
	for rows.Next() {
		var item GrantOriginRow
		if err := rows.Scan(&item.DatabaseName, &item.AccountName, &item.ObjectType, &item.ObjectName,
			&item.Privilege, &item.OriginKind, &item.ViaChain, &item.InheritPath, &item.SetPath); err != nil {
			return nil, err
		}
		item.OriginLabel = postgres.OriginLabelPT(item.OriginKind, item.ViaChain)
		out = append(out, item)
	}
	return out, rows.Err()
}

// ColumnGrantRow is a persisted column privilege (own matrix line).
type ColumnGrantRow struct {
	DatabaseName string   `json:"database_name"`
	SchemaName   string   `json:"schema_name"`
	TableName    string   `json:"table_name"`
	ColumnName   string   `json:"column_name"`
	AccountName  string   `json:"account_name"`
	Privilege    string   `json:"privilege"`
	OriginKind   string   `json:"origin_kind"`
	ViaChain     []string `json:"via_chain"`
	OriginLabel  string   `json:"origin_label"`
}

func (s *Store) ListColumnGrants(ctx context.Context, env, run string) ([]ColumnGrantRow, error) {
	rows, err := s.pool.Query(ctx, `SELECT database_name,schema_name,table_name,column_name,account_name,privilege,origin_kind,via_chain
FROM column_grant_snapshot WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid
ORDER BY schema_name,table_name,column_name,account_name,privilege`, env, run)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ColumnGrantRow, 0)
	for rows.Next() {
		var item ColumnGrantRow
		if err := rows.Scan(&item.DatabaseName, &item.SchemaName, &item.TableName, &item.ColumnName,
			&item.AccountName, &item.Privilege, &item.OriginKind, &item.ViaChain); err != nil {
			return nil, err
		}
		item.OriginLabel = postgres.OriginLabelPT(item.OriginKind, item.ViaChain)
		out = append(out, item)
	}
	return out, rows.Err()
}
