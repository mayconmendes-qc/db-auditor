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
	err = conn.QueryRow(ctx, `SELECT EXISTS (
  SELECT 1 FROM information_schema.role_table_grants
  WHERE grantee IN (current_user, 'PUBLIC') AND privilege_type IN ('INSERT','UPDATE','DELETE','TRUNCATE','REFERENCES','TRIGGER')
) OR EXISTS (SELECT 1 FROM pg_roles WHERE rolname = current_user AND (rolsuper OR rolcreatedb OR rolcreaterole))`).Scan(&role.CanWrite)
	if err != nil {
		role.CheckFailed = true
	}
	return role, nil
}
