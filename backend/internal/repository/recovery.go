package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

var ErrRecoveryTarget = errors.New("active operator account not found")

// RecoverOperatorPassword is only called by the local terminal command when
// normal administrative access has been lost. It never creates a new operator.
func (s *Store) RecoverOperatorPassword(ctx context.Context, username, passwordHash string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var id string
	err = tx.QueryRow(ctx, `SELECT id::text FROM auditor_user WHERE username=$1 AND role='operator' AND active FOR UPDATE`, username).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrRecoveryTarget
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE auditor_user SET password_hash=$2,updated_at=now() WHERE id=$1::uuid`, id, passwordHash); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM auditor_session WHERE user_id=$1::uuid`, id); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO auditor_operation_log(username,action,resource_id,result) VALUES('recovery/local','operator.password_recovery',$1,'success')`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
