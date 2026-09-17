package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// CollectServer gathers server-level facts using a read-only query.
func CollectServer(ctx context.Context, conn *pgx.Conn) (ServerFacts, error) {
	var (
		facts     ServerFacts
		uptimeRaw time.Duration
	)
	err := conn.QueryRow(ctx, serverSQL).Scan(
		&facts.ServerVersion,
		&facts.StartedAt,
		&uptimeRaw,
		&facts.Timezone,
		&facts.ServerEncoding,
		&facts.MaxConnections,
		&facts.CurrentConnections,
		&facts.Autovacuum,
	)
	if err != nil {
		return ServerFacts{}, fmt.Errorf("server collector: %w", err)
	}
	facts.Uptime = uptimeRaw
	return facts, nil
}
