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

type Config struct {
	HTTPAddress string
	Database    Database
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
	}
	if cfg.Database.URL == "" {
		return Config{}, fmt.Errorf("AUDITOR_DATABASE_URL is required")
	}
	if _, err := url.ParseRequestURI(cfg.Database.URL); err != nil {
		return Config{}, fmt.Errorf("AUDITOR_DATABASE_URL is invalid: %w", err)
	}
	return cfg, nil
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
