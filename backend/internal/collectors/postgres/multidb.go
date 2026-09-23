package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/mayconmendes-qc/timescale-auditor/internal/config"
)

// DatabaseVisitor is called once per connectable user database.
// Returning an error records a PartialError for that database and continues.
type DatabaseVisitor func(ctx context.Context, conn *pgx.Conn, databaseName string) error

// ForEachUserDatabase lists databases from baseURL, then connects to each
// non-template, connectable database in scope. Failures on individual databases
// are accumulated as PartialError and do not abort the loop.
// A fatal error is only returned when the initial connection or database list fails.
func ForEachUserDatabase(
	ctx context.Context,
	baseURL string,
	scope config.Scope,
	visit DatabaseVisitor,
) (partial []PartialError, err error) {
	if strings.TrimSpace(baseURL) == "" {
		return nil, fmt.Errorf("base DSN is empty")
	}
	if visit == nil {
		return nil, fmt.Errorf("database visitor is nil")
	}

	root, err := pgx.Connect(ctx, baseURL)
	if err != nil {
		return nil, fmt.Errorf("connect target: %w", err)
	}
	defer func() { _ = root.Close(ctx) }()

	databases, err := CollectDatabases(ctx, root, scope)
	if err != nil {
		return nil, err
	}

	for _, db := range databases {
		if ctx.Err() != nil {
			partial = append(partial, PartialError{
				Database: db.Name,
				Op:       "cancelled",
				Message:  ctx.Err().Error(),
			})
			break
		}
		if db.IsTemplate || !db.AllowConnections {
			continue
		}
		dbURL, err := rewriteDatabase(baseURL, db.Name)
		if err != nil {
			partial = append(partial, PartialError{
				Database: db.Name, Op: "rewrite_url", Message: err.Error(),
			})
			continue
		}
		dbConn, err := pgx.Connect(ctx, dbURL)
		if err != nil {
			partial = append(partial, PartialError{
				Database: db.Name, Op: "connect", Message: err.Error(),
			})
			continue
		}
		visitErr := visit(ctx, dbConn, db.Name)
		_ = dbConn.Close(ctx)
		if visitErr != nil {
			partial = append(partial, PartialError{
				Database: db.Name, Op: "collect", Message: visitErr.Error(),
			})
			continue
		}
	}
	return partial, nil
}

// FormatPartialErrors returns a short human-readable summary of partial errors.
func FormatPartialErrors(partial []PartialError, max int) string {
	if len(partial) == 0 {
		return ""
	}
	if max <= 0 {
		max = 5
	}
	n := len(partial)
	if n > max {
		n = max
	}
	parts := make([]string, 0, n)
	for i := 0; i < n; i++ {
		p := partial[i]
		parts = append(parts, fmt.Sprintf("%s[%s]: %s", p.Database, p.Op, p.Message))
	}
	s := strings.Join(parts, "; ")
	if len(partial) > max {
		s += fmt.Sprintf(" (+%d)", len(partial)-max)
	}
	return s
}
