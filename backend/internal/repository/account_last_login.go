package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	AccountLastLoginSourceLog  = "log"
	MaxAccountLastLoginEntries = 5000
)

// AccountLastLoginObservation is the stored aggregate (role + time only).
type AccountLastLoginObservation struct {
	EnvironmentID string    `json:"environment_id"`
	RoleName      string    `json:"role"`
	LastLoginAt   time.Time `json:"last_login_at"`
	Source        string    `json:"source"`
	TokenID       *string   `json:"token_id,omitempty"`
	ReceivedAt    time.Time `json:"received_at"`
}

// AccountLastLoginInput is one closed entry from the CLI (ADR-0001 D10).
type AccountLastLoginInput struct {
	Role        string    `json:"role"`
	LastLoginAt time.Time `json:"last_login_at"`
}

var (
	ErrAccountLastLoginValidation = errors.New("account last login validation")
	ErrAccountLastLoginTooMany    = errors.New("account last login too many entries")
)

// UpsertAccountLastLogins keeps the latest last_login_at per role for the environment.
func (s *Store) UpsertAccountLastLogins(ctx context.Context, env string, tokenID *string, entries []AccountLastLoginInput) (int, error) {
	if len(entries) == 0 {
		return 0, nil
	}
	if len(entries) > MaxAccountLastLoginEntries {
		return 0, ErrAccountLastLoginTooMany
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	upserted := 0
	for _, e := range entries {
		role := strings.TrimSpace(e.Role)
		if role == "" || len(role) > 128 || e.LastLoginAt.IsZero() {
			return 0, ErrAccountLastLoginValidation
		}
		var tid any
		if tokenID != nil && *tokenID != "" {
			tid = *tokenID
		}
		tag, err := tx.Exec(ctx, `
INSERT INTO account_last_login_observation (environment_id, role_name, last_login_at, source, token_id, received_at)
VALUES ($1::uuid, $2, $3, 'log', $4::uuid, now())
ON CONFLICT (environment_id, role_name) DO UPDATE SET
  last_login_at = CASE
    WHEN EXCLUDED.last_login_at >= account_last_login_observation.last_login_at THEN EXCLUDED.last_login_at
    ELSE account_last_login_observation.last_login_at END,
  token_id = CASE
    WHEN EXCLUDED.last_login_at >= account_last_login_observation.last_login_at THEN EXCLUDED.token_id
    ELSE account_last_login_observation.token_id END,
  received_at = CASE
    WHEN EXCLUDED.last_login_at >= account_last_login_observation.last_login_at THEN EXCLUDED.received_at
    ELSE account_last_login_observation.received_at END,
  source = 'log'`, env, role, e.LastLoginAt.UTC(), tid)
		if err != nil {
			return 0, err
		}
		upserted += int(tag.RowsAffected())
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return upserted, nil
}

// ListAccountLastLogins returns observations for an environment (role + time + source).
func (s *Store) ListAccountLastLogins(ctx context.Context, env string) ([]AccountLastLoginObservation, error) {
	rows, err := s.pool.Query(ctx, `
SELECT environment_id::text, role_name, last_login_at, source, token_id::text, received_at
FROM account_last_login_observation
WHERE environment_id=$1::uuid
ORDER BY role_name`, env)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AccountLastLoginObservation, 0)
	for rows.Next() {
		var o AccountLastLoginObservation
		var tid *string
		if err := rows.Scan(&o.EnvironmentID, &o.RoleName, &o.LastLoginAt, &o.Source, &tid, &o.ReceivedAt); err != nil {
			return nil, err
		}
		o.TokenID = tid
		out = append(out, o)
	}
	return out, rows.Err()
}

// MapAccountLastLoginsByRole returns last_login_at and source keyed by role name.
func (s *Store) MapAccountLastLoginsByRole(ctx context.Context, env string) (map[string]AccountLastLoginObservation, error) {
	items, err := s.ListAccountLastLogins(ctx, env)
	if err != nil {
		return nil, err
	}
	out := make(map[string]AccountLastLoginObservation, len(items))
	for _, o := range items {
		out[o.RoleName] = o
	}
	return out, nil
}

// GetAccountLastLogin looks up one role (test helper / analyzer).
func (s *Store) GetAccountLastLogin(ctx context.Context, env, role string) (*AccountLastLoginObservation, error) {
	var o AccountLastLoginObservation
	var tid *string
	err := s.pool.QueryRow(ctx, `
SELECT environment_id::text, role_name, last_login_at, source, token_id::text, received_at
FROM account_last_login_observation
WHERE environment_id=$1::uuid AND role_name=$2`, env, role).Scan(
		&o.EnvironmentID, &o.RoleName, &o.LastLoginAt, &o.Source, &tid, &o.ReceivedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	o.TokenID = tid
	return &o, nil
}
