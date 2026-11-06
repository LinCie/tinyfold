package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tinyfold/internal/database"
	"tinyfold/internal/module/auth"
)

func newAuthRepository(t *testing.T) (*sqlx.DB, auth.Repository) {
	t.Helper()
	db, err := database.NewDatabase(":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	require.NoError(t, goose.SetDialect("sqlite"))
	require.NoError(t, goose.UpContext(context.Background(), db.DB, "../../../migrations"))
	_, err = db.Exec(`INSERT INTO users (id, username, email, display_name, bio, role, status, created_at, updated_at)
		VALUES ('user-1', 'test-user', 'test@example.com', 'Test User', '', 'author', 'active', ?, ?)`, time.Now(), time.Now())
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, db.Close()) })
	return db, auth.NewSQLiteRepository(db)
}

func TestSQLiteRepository_CreateRefreshToken(t *testing.T) {
	db, repo := newAuthRepository(t)
	expires := time.Now().UTC().Add(time.Hour)

	err := repo.CreateRefreshToken(context.Background(), "refresh-1", "user-1", expires)

	require.NoError(t, err)
	var got struct {
		TokenID   string    `db:"token_id"`
		UserID    string    `db:"user_id"`
		ExpiresAt time.Time `db:"expires_at"`
	}
	require.NoError(t, db.Get(&got, `SELECT token_id, user_id, expires_at FROM refresh_tokens WHERE token_id = ?`, "refresh-1"))
	assert.Equal(t, "user-1", got.UserID)
	assert.True(t, got.ExpiresAt.Equal(expires))
}

func TestSQLiteRepository_RotateRefreshTokenIsOneTimeAndAtomic(t *testing.T) {
	db, repo := newAuthRepository(t)
	ctx := context.Background()
	now := time.Now().UTC()
	require.NoError(t, repo.CreateRefreshToken(ctx, "old", "user-1", now.Add(time.Hour)))
	require.NoError(t, repo.CreateRefreshToken(ctx, "collision", "user-1", now.Add(time.Hour)))

	// A failed insert must roll back revoking the old token.
	err := repo.RotateRefreshToken(ctx, "old", "user-1", "collision", now.Add(2*time.Hour), now)
	require.Error(t, err)
	var revokedAt *time.Time
	require.NoError(t, db.Get(&revokedAt, `SELECT revoked_at FROM refresh_tokens WHERE token_id = 'old'`))
	assert.Nil(t, revokedAt)

	require.NoError(t, repo.RotateRefreshToken(ctx, "old", "user-1", "new", now.Add(2*time.Hour), now))
	assert.ErrorIs(t, repo.RotateRefreshToken(ctx, "old", "user-1", "another", now.Add(3*time.Hour), now), auth.ErrInvalidRefreshToken)

	var count int
	require.NoError(t, db.Get(&count, `SELECT count(*) FROM refresh_tokens WHERE token_id = 'new'`))
	assert.Equal(t, 1, count)
}

func TestSQLiteRepository_RotateRefreshTokenRejectsExpiredAndWrongOwner(t *testing.T) {
	_, repo := newAuthRepository(t)
	ctx := context.Background()
	now := time.Now().UTC()
	require.NoError(t, repo.CreateRefreshToken(ctx, "expired", "user-1", now.Add(-time.Second)))
	require.NoError(t, repo.CreateRefreshToken(ctx, "owned", "user-1", now.Add(time.Hour)))

	assert.ErrorIs(t, repo.RotateRefreshToken(ctx, "expired", "user-1", "new-1", now.Add(time.Hour), now), auth.ErrInvalidRefreshToken)
	assert.ErrorIs(t, repo.RotateRefreshToken(ctx, "owned", "another-user", "new-2", now.Add(time.Hour), now), auth.ErrInvalidRefreshToken)
}

func TestSQLiteRepository_RevokeRefreshToken(t *testing.T) {
	db, repo := newAuthRepository(t)
	ctx := context.Background()
	now := time.Now().UTC()
	require.NoError(t, repo.CreateRefreshToken(ctx, "refresh-1", "user-1", now.Add(time.Hour)))

	require.NoError(t, repo.RevokeRefreshToken(ctx, "refresh-1", "user-1", now))
	var revokedAt *time.Time
	require.NoError(t, db.Get(&revokedAt, `SELECT revoked_at FROM refresh_tokens WHERE token_id = 'refresh-1'`))
	assert.NotNil(t, revokedAt)

	// Revoking an unknown token is intentionally idempotent.
	assert.NoError(t, repo.RevokeRefreshToken(ctx, "missing", "user-1", now))
	assert.False(t, errors.Is(repo.RevokeRefreshToken(ctx, "refresh-1", "another-user", now), auth.ErrInvalidRefreshToken))
}
