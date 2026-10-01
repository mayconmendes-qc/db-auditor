// Package config loads and validates process configuration without logging secrets.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

type Database struct {
	URL              string
	StatementTimeout time.Duration
	LockTimeout      time.Duration
	ApplicationName  string
}

// Scope filters which databases/schemas collectors may touch on audited environments.
type Scope struct {
	DatabaseAllowlist []string
	DatabaseDenylist  []string
	SchemaAllowlist   []string
	SchemaDenylist    []string
}

type Config struct {
	HTTPAddress string
	Database    Database
	Scope       Scope
	Collection  CollectionPolicy
}

// CollectionPolicy limits optional, potentially expensive collectors.
type CollectionPolicy struct {
	ColumnStatsEnabled bool
	ColumnStatsLimit   int
	ColumnStatsSchemas []string
	WorkloadEnabled    bool
	WorkloadLimit      int
}

func Load() (Config, error) {
	statementTimeout, err := durationFromEnv("AUDITOR_STATEMENT_TIMEOUT", 15*time.Second)
	if err != nil {
		return Config{}, err
	}
	lockTimeout, err := durationFromEnv("AUDITOR_LOCK_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		HTTPAddress: env("AUDITOR_HTTP_ADDRESS", ":8080"),
		Database: Database{
			URL:              env("AUDITOR_DATABASE_URL", ""),
			StatementTimeout: statementTimeout,
			LockTimeout:      lockTimeout,
			ApplicationName:  env("AUDITOR_APPLICATION_NAME", "timescale-auditor"),
		},
		Scope: Scope{
			DatabaseAllowlist: splitCSV(env("AUDITOR_DATABASE_ALLOWLIST", "")),
			DatabaseDenylist:  splitCSV(env("AUDITOR_DATABASE_DENYLIST", "template0,template1,postgres")),
			SchemaAllowlist:   splitCSV(env("AUDITOR_SCHEMA_ALLOWLIST", "")),
			SchemaDenylist:    splitCSV(env("AUDITOR_SCHEMA_DENYLIST", "pg_catalog,information_schema")),
		},
		Collection: CollectionPolicy{
			ColumnStatsEnabled: boolFromEnv("AUDITOR_COLUMN_STATS_ENABLED", true),
			ColumnStatsLimit:   intFromEnv("AUDITOR_COLUMN_STATS_LIMIT", 5000),
			ColumnStatsSchemas: splitCSV(env("AUDITOR_COLUMN_STATS_SCHEMA_ALLOWLIST", "")),
			WorkloadEnabled:    boolFromEnv("AUDITOR_WORKLOAD_ENABLED", true),
			WorkloadLimit:      intFromEnv("AUDITOR_WORKLOAD_LIMIT", 500),
		},
	}
	if cfg.Database.URL == "" {
		return Config{}, fmt.Errorf("AUDITOR_DATABASE_URL is required")
	}
	if _, err := url.ParseRequestURI(cfg.Database.URL); err != nil {
		return Config{}, fmt.Errorf("AUDITOR_DATABASE_URL is invalid: %w", err)
	}
	return cfg, nil
}

func boolFromEnv(key string, fallback bool) bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if raw == "" {
		return fallback
	}
	return raw == "1" || raw == "true" || raw == "yes" || raw == "on"
}

func intFromEnv(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	var value int
	if _, err := fmt.Sscanf(raw, "%d", &value); err != nil || value <= 0 {
		return fallback
	}
	if value > 5000 {
		return 5000
	}
	return value
}

// AllowsDatabase reports whether a database name may be collected.
// Denylist wins; empty allowlist means all non-denied names are allowed.
func (s Scope) AllowsDatabase(name string) bool {
	name = strings.TrimSpace(name)
	for _, denied := range s.DatabaseDenylist {
		if strings.EqualFold(denied, name) {
			return false
		}
	}
	if len(s.DatabaseAllowlist) == 0 {
		return true
	}
	for _, allowed := range s.DatabaseAllowlist {
		if strings.EqualFold(allowed, name) {
			return true
		}
	}
	return false
}

// AllowsSchema reports whether a schema name may be collected.
func (s Scope) AllowsSchema(name string) bool {
	name = strings.TrimSpace(name)
	for _, denied := range s.SchemaDenylist {
		if strings.EqualFold(denied, name) {
			return false
		}
	}
	if len(s.SchemaAllowlist) == 0 {
		return true
	}
	for _, allowed := range s.SchemaAllowlist {
		if strings.EqualFold(allowed, name) {
			return true
		}
	}
	return false
}

func SanitizeConnectionString(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "[invalid connection string]"
	}
	if u.User != nil {
		u.User = url.User(u.User.Username())
	}
	q := u.Query()
	for _, key := range []string{"password", "sslkey", "sslcert"} {
		if q.Has(key) {
			q.Set(key, "[redacted]")
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func durationFromEnv(key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", key)
	}
	return value, nil
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func splitCSV(raw string) []string {
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
