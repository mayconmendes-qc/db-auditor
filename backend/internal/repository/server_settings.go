package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

// SaveServerSettings stores the closed GUC set and server_version for one database.
func (s *Store) SaveServerSettings(ctx context.Context, environmentID, auditRunID pgtype.UUID, database string, settings map[string]string) error {
	for name, value := range settings {
		if name == "" || value == "" {
			continue
		}
		_, err := s.pool.Exec(ctx, `INSERT INTO server_setting_snapshot (audit_run_id, environment_id, database_name, setting_name, setting_value)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (audit_run_id, database_name, setting_name) DO UPDATE SET setting_value = EXCLUDED.setting_value, collected_at = now()`,
			auditRunID, environmentID, database, name, value)
		if err != nil {
			return fmt.Errorf("save setting %s: %w", name, err)
		}
	}
	return nil
}
