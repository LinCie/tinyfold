package auth

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
)

type Repository interface {
	CreateRefreshToken(ctx context.Context, tokenID, userID string, expiresAt time.Time) error
	RotateRefreshToken(ctx context.Context, oldTokenID, userID, newTokenID string, newExpiresAt, now time.Time) error
	RevokeRefreshToken(ctx context.Context, tokenID, userID string, revokedAt time.Time) error
}

type sqliteRepository struct {
	db *sqlx.DB
}

func NewSQLiteRepository(db *sqlx.DB) Repository {
	return &sqliteRepository{db: db}
}

func (r *sqliteRepository) CreateRefreshToken(ctx context.Context, tokenID, userID string, expiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO refresh_tokens (token_id, user_id, expires_at)
		VALUES (?, ?, ?)`, tokenID, userID, expiresAt)
	return err
}

func (r *sqliteRepository) RotateRefreshToken(ctx context.Context, oldTokenID, userID, newTokenID string, newExpiresAt, now time.Time) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `UPDATE refresh_tokens SET revoked_at = ?
		WHERE token_id = ? AND user_id = ? AND revoked_at IS NULL AND expires_at > ?`,
		now, oldTokenID, userID, now)
	if err != nil {
		return err
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if updated != 1 {
		return ErrInvalidRefreshToken
	}

	_, err = tx.ExecContext(ctx, `INSERT INTO refresh_tokens (token_id, user_id, expires_at)
		VALUES (?, ?, ?)`, newTokenID, userID, newExpiresAt)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *sqliteRepository) RevokeRefreshToken(ctx context.Context, tokenID, userID string, revokedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE refresh_tokens SET revoked_at = ?
		WHERE token_id = ? AND user_id = ? AND revoked_at IS NULL`, revokedAt, tokenID, userID)
	return err
}
