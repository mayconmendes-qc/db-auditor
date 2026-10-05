package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// AuditorRole is the privilege picture of the session used by the auditor.
type AuditorRole struct {
	Database    string
	RoleName    string
	Superuser   bool
	CreateDB    bool
	CreateRole  bool
	Replication bool
	CanWrite    bool
	CheckFailed bool
}

func CollectAuditorRole(ctx context.Context, conn *pgx.Conn) (AuditorRole, error) {
	var role AuditorRole
	err := conn.QueryRow(ctx, `SELECT current_database(), current_user, r.rolsuper, r.rolcreatedb, r.rolcreaterole, r.rolreplication
FROM pg_roles r WHERE r.rolname = current_user`).Scan(&role.Database, &role.RoleName, &role.Superuser, &role.CreateDB, &role.CreateRole, &role.Replication)
	if err != nil {
		role.CheckFailed = true
		return role, nil
	}
	err = conn.QueryRow(ctx, `SELECT
  r.rolsuper OR r.rolcreatedb OR r.rolcreaterole OR EXISTS (
    SELECT 1
    FROM pg_class c
    JOIN pg_namespace n ON n.oid = c.relnamespace
    WHERE c.relkind IN ('r', 'p')
      AND n.nspname NOT IN ('pg_catalog', 'information_schema', '_timescaledb_catalog', '_timescaledb_config', '_timescaledb_internal')
      AND n.nspname NOT LIKE 'pg_%'
      AND (
        has_table_privilege(c.oid, 'INSERT')
        OR has_table_privilege(c.oid, 'UPDATE')
        OR has_table_privilege(c.oid, 'DELETE')
        OR has_table_privilege(c.oid, 'TRUNCATE')
      )
  )
FROM pg_roles r WHERE r.rolname = current_user`).Scan(&role.CanWrite)
	if err != nil {
		role.CheckFailed = true
	}
	return role, nil
}
