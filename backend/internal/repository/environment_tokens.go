package repository

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// KnownEnvTokenScopes is the closed scope list (ADR-0001 D1).
var KnownEnvTokenScopes = []string{
	"snapshot-context:read",
	"migration-analysis:write",
	"deploy-annotation:write",
	"quality-contract:run",
	"restore-evidence:write",
	"last-login:write",
}

// EnvAPIToken is the metadata view of an environment API token (never the secret).
type EnvAPIToken struct {
	ID            string     `json:"id"`
	EnvironmentID string     `json:"environment_id"`
	Name          string     `json:"name"`
	PublicPrefix  string     `json:"public_prefix"`
	Scopes        []string   `json:"scopes"`
	CreatedBy     *string    `json:"created_by,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	ExpiresAt     time.Time  `json:"expires_at"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty"`
	LastUsedAt    *time.Time `json:"last_used_at,omitempty"`
}

// EnvTokenPrincipal is the authenticated CLI principal for one environment.
type EnvTokenPrincipal struct {
	ID            string
	EnvironmentID string
	Name          string
	PublicPrefix  string
	Scopes        []string
}

// HasScope reports whether the principal includes scope.
func (p *EnvTokenPrincipal) HasScope(scope string) bool {
	if p == nil {
		return false
	}
	for _, s := range p.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// CreateEnvAPITokenParams is the input for minting a token.
type CreateEnvAPITokenParams struct {
	EnvironmentID string
	Name          string
	Scopes        []string
	CreatedBy     string
	ExpiresAt     time.Time
}

// CreateEnvAPITokenResult returns metadata plus the plaintext once.
type CreateEnvAPITokenResult struct {
	Token     EnvAPIToken
	Plaintext string `json:"token"`
}

func validScopes(scopes []string) error {
	if len(scopes) == 0 {
		return errors.New("informe ao menos um escopo")
	}
	known := map[string]bool{}
	for _, s := range KnownEnvTokenScopes {
		known[s] = true
	}
	seen := map[string]bool{}
	for _, s := range scopes {
		s = strings.TrimSpace(s)
		if !known[s] {
			return fmt.Errorf("escopo desconhecido %q", s)
		}
		if seen[s] {
			return errors.New("escopos duplicados")
		}
		seen[s] = true
	}
	return nil
}

// CreateEnvAPIToken mints dba_<prefix>_<secret>, stores only sha256.
func (s *Store) CreateEnvAPIToken(ctx context.Context, p CreateEnvAPITokenParams) (*CreateEnvAPITokenResult, error) {
	name := strings.TrimSpace(p.Name)
	if name == "" || len(name) > 128 {
		return nil, errors.New("nome inválido")
	}
	if err := validScopes(p.Scopes); err != nil {
		return nil, err
	}
	if p.ExpiresAt.IsZero() || !p.ExpiresAt.After(time.Now().UTC()) {
		return nil, errors.New("expires_at obrigatório e no futuro")
	}
	prefixBytes := make([]byte, 5)
	if _, err := rand.Read(prefixBytes); err != nil {
		return nil, err
	}
	prefix := hex.EncodeToString(prefixBytes)[:8]
	secret := make([]byte, 24)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	secretStr := base64.RawURLEncoding.EncodeToString(secret)
	plaintext := "dba_" + prefix + "_" + secretStr
	if len(plaintext) > 120 {
		return nil, errors.New("token gerado excede o limite do Bearer")
	}
	sum := sha256.Sum256([]byte(plaintext))
	var id string
	var createdAt, expiresAt time.Time
	var createdBy *string
	err := s.pool.QueryRow(ctx, `
INSERT INTO environment_api_token(environment_id,name,public_prefix,token_sha256,scopes,created_by,expires_at)
VALUES($1::uuid,$2,$3,$4,$5,$6::uuid,$7)
RETURNING id::text, created_at, expires_at, created_by::text`,
		p.EnvironmentID, name, prefix, sum[:], p.Scopes, nullUUID(p.CreatedBy), p.ExpiresAt.UTC(),
	).Scan(&id, &createdAt, &expiresAt, &createdBy)
	if err != nil {
		return nil, err
	}
	tok := EnvAPIToken{
		ID: id, EnvironmentID: p.EnvironmentID, Name: name, PublicPrefix: prefix,
		Scopes: append([]string{}, p.Scopes...), CreatedBy: createdBy,
		CreatedAt: createdAt.UTC(), ExpiresAt: expiresAt.UTC(),
	}
	return &CreateEnvAPITokenResult{Token: tok, Plaintext: plaintext}, nil
}

func nullUUID(id string) any {
	if id == "" {
		return nil
	}
	return id
}

// ListEnvAPITokens returns metadata only.
func (s *Store) ListEnvAPITokens(ctx context.Context, environmentID string) ([]EnvAPIToken, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id::text, environment_id::text, name, public_prefix, scopes, created_by::text,
       created_at, expires_at, revoked_at, last_used_at
FROM environment_api_token
WHERE environment_id=$1::uuid
ORDER BY created_at DESC`, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]EnvAPIToken, 0)
	for rows.Next() {
		var t EnvAPIToken
		if err := rows.Scan(&t.ID, &t.EnvironmentID, &t.Name, &t.PublicPrefix, &t.Scopes,
			&t.CreatedBy, &t.CreatedAt, &t.ExpiresAt, &t.RevokedAt, &t.LastUsedAt); err != nil {
			return nil, err
		}
		t.CreatedAt = t.CreatedAt.UTC()
		t.ExpiresAt = t.ExpiresAt.UTC()
		items = append(items, t)
	}
	return items, rows.Err()
}

// RevokeEnvAPIToken sets revoked_at immediately.
func (s *Store) RevokeEnvAPIToken(ctx context.Context, environmentID, tokenID string) error {
	tag, err := s.pool.Exec(ctx, `
UPDATE environment_api_token SET revoked_at=now()
WHERE id=$1::uuid AND environment_id=$2::uuid AND revoked_at IS NULL`, tokenID, environmentID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// ResolveEnvAPIToken authenticates a Bearer plaintext token.
func (s *Store) ResolveEnvAPIToken(ctx context.Context, plaintext string) (*EnvTokenPrincipal, error) {
	if !strings.HasPrefix(plaintext, "dba_") {
		return nil, nil
	}
	sum := sha256.Sum256([]byte(plaintext))
	var p EnvTokenPrincipal
	var expiresAt time.Time
	var revokedAt *time.Time
	err := s.pool.QueryRow(ctx, `
SELECT id::text, environment_id::text, name, public_prefix, scopes, expires_at, revoked_at
FROM environment_api_token WHERE token_sha256=$1`, sum[:]).Scan(
		&p.ID, &p.EnvironmentID, &p.Name, &p.PublicPrefix, &p.Scopes, &expiresAt, &revokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if revokedAt != nil || time.Now().UTC().After(expiresAt.UTC()) {
		return nil, nil
	}
	_, _ = s.pool.Exec(ctx, `UPDATE environment_api_token SET last_used_at=now() WHERE id=$1::uuid`, p.ID)
	return &p, nil
}

// LogEnvTokenOperation writes auditor_operation_log with token id, never the secret.
func (s *Store) LogEnvTokenOperation(ctx context.Context, tokenID, action, environmentID, resourceID, result string) error {
	username := "env-token:" + tokenID
	var env any
	if environmentID != "" {
		env = environmentID
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO auditor_operation_log(user_id,username,action,environment_id,resource_id,result)
		VALUES(NULL,$1,$2,$3,$4,$5)`, username, action, env, resourceID, result)
	return err
}
