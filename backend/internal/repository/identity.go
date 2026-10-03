package repository

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"crypto/pbkdf2"

	"github.com/jackc/pgx/v5"
)

const passwordIterations = 600000

type AuditorUser struct {
	ID           string   `json:"id"`
	Username     string   `json:"username"`
	Role         string   `json:"role"`
	Environments []string `json:"environments"`
	Active       bool     `json:"-"`
	PasswordHash string   `json:"-"`
}

func HashAuditorPassword(password string) (string, error) {
	if len(password) < 16 || len(password) > 1024 {
		return "", errors.New("password must be 16-1024 bytes")
	}
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, passwordIterations, 32)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", passwordIterations,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

func CheckAuditorPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" || parts[1] != "600000" {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil || len(salt) != 32 {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(want) != 32 {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, passwordIterations, 32)
	return err == nil && subtle.ConstantTimeCompare(got, want) == 1
}

func (s *Store) BootstrapOperator(ctx context.Context, username, password string) error {
	if username == "" || password == "" {
		return errors.New("bootstrap username and password are required")
	}
	hash, err := HashAuditorPassword(password)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO auditor_user(username,password_hash,role)
		SELECT $1,$2,'operator' WHERE NOT EXISTS (SELECT 1 FROM auditor_user)`, username, hash)
	return err
}

func (s *Store) HasAuditorUsers(ctx context.Context) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM auditor_user)`).Scan(&exists)
	return exists, err
}

func (s *Store) CreateAuditorUser(ctx context.Context, username, passwordHash, role string, environments []string) (*AuditorUser, error) {
	if role != "viewer" && role != "auditor" && role != "operator" {
		return nil, errors.New("invalid role")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	user := &AuditorUser{Environments: environments}
	err = tx.QueryRow(ctx, `INSERT INTO auditor_user(username,password_hash,role) VALUES($1,$2,$3)
		RETURNING id::text,username,role,active`, username, passwordHash, role).
		Scan(&user.ID, &user.Username, &user.Role, &user.Active)
	if err != nil {
		return nil, err
	}
	for _, id := range environments {
		if _, err := tx.Exec(ctx, `INSERT INTO auditor_user_environment(user_id,environment_id) VALUES($1,$2)`, user.ID, id); err != nil {
			return nil, err
		}
	}
	return user, tx.Commit(ctx)
}

func (s *Store) FindAuditorUser(ctx context.Context, username string) (*AuditorUser, error) {
	user := &AuditorUser{}
	err := s.pool.QueryRow(ctx, `SELECT id::text,username,role,active,password_hash FROM auditor_user WHERE username=$1`, username).
		Scan(&user.ID, &user.Username, &user.Role, &user.Active, &user.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.loadAuditorEnvironments(ctx, user)
}

func (s *Store) loadAuditorEnvironments(ctx context.Context, user *AuditorUser) (*AuditorUser, error) {
	rows, err := s.pool.Query(ctx, `SELECT environment_id::text FROM auditor_user_environment WHERE user_id=$1 ORDER BY environment_id`, user.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	user.Environments = []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		user.Environments = append(user.Environments, id)
	}
	return user, rows.Err()
}

func (s *Store) CreateAuditorSession(ctx context.Context, userID string, tokenHash []byte, expiry time.Time) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO auditor_session(token_hash,user_id,expires_at) VALUES($1,$2,$3)`, tokenHash, userID, expiry)
	return err
}

func (s *Store) GetAuditorSession(ctx context.Context, tokenHash []byte) (*AuditorUser, error) {
	user := &AuditorUser{}
	err := s.pool.QueryRow(ctx, `SELECT u.id::text,u.username,u.role,u.active,u.password_hash
		FROM auditor_session s JOIN auditor_user u ON u.id=s.user_id
		WHERE s.token_hash=$1 AND s.expires_at>now() AND u.active`, tokenHash).
		Scan(&user.ID, &user.Username, &user.Role, &user.Active, &user.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.loadAuditorEnvironments(ctx, user)
}

func (s *Store) DeleteAuditorSession(ctx context.Context, tokenHash []byte) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM auditor_session WHERE token_hash=$1`, tokenHash)
	return err
}

func (s *Store) DeleteExpiredAuditorSessions(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM auditor_session WHERE expires_at <= now()`)
	return err
}

func (s *Store) LogAuditorOperation(ctx context.Context, user *AuditorUser, action, environmentID, resourceID, result string) error {
	var id any
	var username = "anonymous"
	if user != nil {
		id, username = user.ID, user.Username
	}
	var env any
	if environmentID != "" {
		env = environmentID
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO auditor_operation_log(user_id,username,action,environment_id,resource_id,result)
		VALUES($1,$2,$3,$4,$5,$6)`, id, username, action, env, resourceID, result)
	return err
}

// ResolveAuditorResourceEnvironment prevents a guessed resource UUID from
// bypassing the environment allowlist on routes without an environment path.
func (s *Store) ResolveAuditorResourceEnvironment(ctx context.Context, kind, id string) (string, error) {
	var query string
	switch kind {
	case "audit-runs":
		query = `SELECT environment_id::text FROM audit_run WHERE id=$1`
	case "findings":
		query = `SELECT environment_id::text FROM finding WHERE id=$1`
	default:
		return "", errors.New("unsupported resource")
	}
	var environmentID string
	err := s.pool.QueryRow(ctx, query, id).Scan(&environmentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return environmentID, err
}
