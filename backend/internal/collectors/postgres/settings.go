package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ClosedSettingNames is the only GUC set persisted for drift and compare.
var ClosedSettingNames = []string{
	"server_version",
	"shared_buffers",
	"work_mem",
	"max_connections",
	"timescaledb.max_background_workers",
}

// CollectClosedSettings reads server_version and the closed GUC set. Missing names are omitted.
func CollectClosedSettings(ctx context.Context, conn *pgx.Conn) (map[string]string, error) {
	rows, err := conn.Query(ctx, `SELECT name, COALESCE(current_setting(name, true), '')
FROM unnest(ARRAY['server_version','shared_buffers','work_mem','max_connections','timescaledb.max_background_workers']) AS name`)
	if err != nil {
		return nil, fmt.Errorf("closed settings: %w", err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			return nil, err
		}
		if value != "" {
			out[name] = value
		}
	}
	return out, rows.Err()
}
