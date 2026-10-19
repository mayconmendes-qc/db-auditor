package migrationrisk

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// LoadDirectContext reads table metadata via an already-validated read-only connection.
func LoadDirectContext(ctx context.Context, conn *pgx.Conn, tables []string) (Context, error) {
	out := Context{Mode: "direct", Tables: map[string]TableContext{}}
	var ver int
	if err := conn.QueryRow(ctx, `SELECT current_setting('server_version_num')::int`).Scan(&ver); err != nil {
		return out, err
	}
	out.PGVersion = ver / 10000
	if len(tables) == 0 {
		return out, nil
	}
	for _, t := range tables {
		schema, name := splitQualified(t)
		var size, rows int64
		err := conn.QueryRow(ctx, `
SELECT COALESCE(pg_total_relation_size(c.oid),0), COALESCE(c.reltuples::bigint,0)
FROM pg_class c
JOIN pg_namespace n ON n.oid=c.relnamespace
WHERE n.nspname=$1 AND c.relname=$2 AND c.relkind IN ('r','p')`, schema, name).Scan(&size, &rows)
		if err != nil {
			continue
		}
		key := strings.ToLower(schema + "." + name)
		out.Tables[key] = TableContext{Qualified: key, SizeBytes: size, RowEstimate: rows}
	}
	return out, nil
}

func splitQualified(t string) (schema, name string) {
	t = strings.TrimSpace(t)
	parts := strings.Split(t, ".")
	if len(parts) == 1 {
		return "public", parts[0]
	}
	return parts[0], parts[len(parts)-1]
}

// AgeOK reports whether the snapshot age is within the threshold.
func AgeOK(startedAt string, maxDays int, now time.Time) bool {
	if startedAt == "" || maxDays <= 0 {
		return true
	}
	ts, err := time.Parse(time.RFC3339, startedAt)
	if err != nil {
		return false
	}
	return now.Sub(ts) <= time.Duration(maxDays)*24*time.Hour
}

// ParseTablesCSV splits the --tables / query list.
func ParseTablesCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// MaxTablesPerContextCall bounds migration-context API fan-out.
const MaxTablesPerContextCall = 100

// ValidateTablesBound returns an error when the request asks for too many tables.
func ValidateTablesBound(tables []string) error {
	if len(tables) > MaxTablesPerContextCall {
		return fmt.Errorf("no máximo %d tabelas por chamada", MaxTablesPerContextCall)
	}
	return nil
}
