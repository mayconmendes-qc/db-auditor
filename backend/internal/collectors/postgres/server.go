package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// CollectServer gathers server-level facts using a read-only query.
func CollectServer(ctx context.Context, conn *pgx.Conn) (ServerFacts, error) {
	var facts ServerFacts
	err := conn.QueryRow(ctx, serverSQL).Scan(
		&facts.ServerVersion,
		&facts.StartedAt,
		&facts.Uptime,
		&facts.Timezone,
		&facts.ServerEncoding,
		&facts.MaxConnections,
		&facts.CurrentConnections,
		&facts.Autovacuum,
	)
	if err != nil {
		return ServerFacts{}, fmt.Errorf("server collector: %w", err)
	}
	return facts, nil
}
