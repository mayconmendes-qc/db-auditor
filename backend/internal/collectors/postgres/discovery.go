package postgres

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/mayconmendes-qc/timescale-auditor/internal/config"
)

// DiscoveryMode matches audit_environment.discovery_mode.
type DiscoveryMode string

const (
	ModeSingleDatabase DiscoveryMode = "single_database" // Tiger-style: one DB, many schemas
	ModeMultiDatabase  DiscoveryMode = "multi_database"  // Unifique-style: many DBs
)

// DiscoveryResult holds topology facts plus partial errors (T-030).
type DiscoveryResult struct {
	Server    ServerFacts
	Databases []DatabaseFacts
	Schemas   []SchemaFacts
	Errors    []PartialError
}

// DiscoverTopology connects to the target and collects server/database/schema facts.
// In multi_database mode it opens a controlled connection per eligible database for schemas.
// Inaccessible databases are recorded as PartialError and do not abort the whole discovery.
func DiscoverTopology(ctx context.Context, baseURL string, mode DiscoveryMode, scope config.Scope) (DiscoveryResult, error) {
	var result DiscoveryResult

	conn, err := pgx.Connect(ctx, baseURL)
	if err != nil {
		return result, fmt.Errorf("connect target: %w", err)
	}
	defer conn.Close(ctx)

	server, err := CollectServer(ctx, conn)
	if err != nil {
		return result, err
	}
	result.Server = server

	databases, err := CollectDatabases(ctx, conn, scope)
	if err != nil {
		return result, err
	}
	result.Databases = databases

	switch mode {
	case ModeSingleDatabase:
		schemas, err := CollectSchemas(ctx, conn, scope)
		if err != nil {
			return result, err
		}
		result.Schemas = schemas
	case ModeMultiDatabase:
		for _, db := range databases {
			if db.IsTemplate || !db.AllowConnections {
				continue
			}
			dbURL, err := rewriteDatabase(baseURL, db.Name)
			if err != nil {
				result.Errors = append(result.Errors, PartialError{
					Database: db.Name, Op: "rewrite_url", Err: err.Error(),
				})
				continue
			}
			dbConn, err := pgx.Connect(ctx, dbURL)
			if err != nil {
				result.Errors = append(result.Errors, PartialError{
					Database: db.Name, Op: "connect", Err: err.Error(),
				})
				continue
			}
			schemas, err := CollectSchemas(ctx, dbConn, scope)
			_ = dbConn.Close(ctx)
			if err != nil {
				result.Errors = append(result.Errors, PartialError{
					Database: db.Name, Op: "collect_schemas", Err: err.Error(),
				})
				continue
			}
			result.Schemas = append(result.Schemas, schemas...)
		}
	default:
		return result, fmt.Errorf("unsupported discovery mode %q", mode)
	}

	return result, nil
}

func rewriteDatabase(baseURL, database string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	u.Path = "/" + strings.TrimPrefix(database, "/")
	return u.String(), nil
}
