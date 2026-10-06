package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

var ErrAccountNotFound = errors.New("account not found")
var ErrLastOperator = errors.New("last active operator cannot be disabled or demoted")

type AuditorAccount struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Role         string    `json:"role"`
	Active       bool      `json:"active"`
	Environments []string  `json:"environments"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type AccountChange struct {
	Role         string
	Active       bool
	Environments []string
	PasswordHash string
}

func (s *Store) ListAuditorAccounts(ctx context.Context) ([]AuditorAccount, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text,username,role,active,updated_at FROM auditor_user ORDER BY username LIMIT 500`)
	if err != nil {
		return nil, err
	}
	items := []AuditorAccount{}
	for rows.Next() {
		var item AuditorAccount
		if err = rows.Scan(&item.ID, &item.Username, &item.Role, &item.Active, &item.UpdatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for i := range items {
		rows, err = s.pool.Query(ctx, `SELECT environment_id::text FROM auditor_user_environment WHERE user_id=$1::uuid ORDER BY environment_id`, items[i].ID)
		if err != nil {
			return nil, err
		}
		items[i].Environments = []string{}
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			items[i].Environments = append(items[i].Environments, id)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return items, nil
}

// ChangeAuditorPassword revokes every session, including the current one.
func (s *Store) ChangeAuditorPassword(ctx context.Context, userID, passwordHash string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	cmd, err := tx.Exec(ctx, `UPDATE auditor_user SET password_hash=$2,updated_at=now() WHERE id=$1::uuid AND active`, userID, passwordHash)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrAccountNotFound
	}
	if _, err = tx.Exec(ctx, `DELETE FROM auditor_session WHERE user_id=$1::uuid`, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) RevokeAuditorSessions(ctx context.Context, userID string) error {
	cmd, err := s.pool.Exec(ctx, `DELETE FROM auditor_session WHERE user_id=$1::uuid`, userID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		var exists bool
		if err = s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM auditor_user WHERE id=$1::uuid)`, userID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrAccountNotFound
		}
	}
	return nil
}

// UpdateAuditorAccount serializes role changes and preserves an active operator.
// Any account or privilege change revokes sessions so the new policy is applied.
func (s *Store) UpdateAuditorAccount(ctx context.Context, userID string, change AccountChange) (*AuditorAccount, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(842021)`); err != nil {
		return nil, err
	}
	var oldRole string
	var oldActive bool
	err = tx.QueryRow(ctx, `SELECT role,active FROM auditor_user WHERE id=$1::uuid FOR UPDATE`, userID).Scan(&oldRole, &oldActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAccountNotFound
	}
	if err != nil {
		return nil, err
	}
	if oldRole == "operator" && oldActive && (!change.Active || change.Role != "operator") {
		var others int
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM auditor_user WHERE id<>$1::uuid AND role='operator' AND active`, userID).Scan(&others); err != nil {
			return nil, err
		}
		if others == 0 {
			return nil, ErrLastOperator
		}
	}
	var item AuditorAccount
	err = tx.QueryRow(ctx, `UPDATE auditor_user SET role=$2,active=$3,password_hash=COALESCE(NULLIF($4,''),password_hash),updated_at=now()
WHERE id=$1::uuid RETURNING id::text,username,role,active,updated_at`, userID, change.Role, change.Active, change.PasswordHash).
		Scan(&item.ID, &item.Username, &item.Role, &item.Active, &item.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM auditor_user_environment WHERE user_id=$1::uuid`, userID); err != nil {
		return nil, err
	}
	item.Environments = []string{}
	for _, env := range change.Environments {
		if _, err = tx.Exec(ctx, `INSERT INTO auditor_user_environment(user_id,environment_id) VALUES($1::uuid,$2::uuid)`, userID, env); err != nil {
			return nil, err
		}
		item.Environments = append(item.Environments, env)
	}
	if _, err = tx.Exec(ctx, `DELETE FROM auditor_session WHERE user_id=$1::uuid`, userID); err != nil {
		return nil, err
	}
	return &item, tx.Commit(ctx)
}
