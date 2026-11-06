package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tinyfold/internal/config"
	"tinyfold/internal/model"
	"tinyfold/internal/module/auth"
	"tinyfold/internal/pkg/jwt"
)

type stubRepository struct {
	create func(context.Context, string, string, time.Time) error
	rotate func(context.Context, string, string, string, time.Time, time.Time) error
	revoke func(context.Context, string, string, time.Time) error
}

func (r *stubRepository) CreateRefreshToken(ctx context.Context, id, userID string, expires time.Time) error {
	if r.create != nil {
		return r.create(ctx, id, userID, expires)
	}
	return nil
}
func (r *stubRepository) RotateRefreshToken(ctx context.Context, oldID, userID, newID string, expires, now time.Time) error {
	if r.rotate != nil {
		return r.rotate(ctx, oldID, userID, newID, expires, now)
	}
	return nil
}
func (r *stubRepository) RevokeRefreshToken(ctx context.Context, id, userID string, now time.Time) error {
	if r.revoke != nil {
		return r.revoke(ctx, id, userID, now)
	}
	return nil
}

type stubUsers struct {
	create func(context.Context, *model.User) (*model.User, error)
	get    func(context.Context, string) (*model.User, error)
}

func (u *stubUsers) Create(ctx context.Context, user *model.User) (*model.User, error) {
	return u.create(ctx, user)
}
func (u *stubUsers) GetByID(ctx context.Context, id string) (*model.User, error) {
	return u.get(ctx, id)
}

type stubPasswords struct {
	hash   func(context.Context, string) (string, error)
	verify func(context.Context, string, string) (bool, error)
}

func (p *stubPasswords) Hash(ctx context.Context, password string) (string, error) {
	return p.hash(ctx, password)
}
func (p *stubPasswords) Verify(ctx context.Context, password, hash string) (bool, error) {
	return p.verify(ctx, password, hash)
}

func newService(t *testing.T, repo auth.Repository, userService auth.UserService, passwords auth.PasswordService) auth.Service {
	t.Helper()
	service, err := auth.NewService(repo, userService, passwords, &config.Config{
		JWTAccessSecret:  "access-secret-for-auth-tests-at-least-32",
		JWTRefreshSecret: "refresh-secret-for-auth-tests-at-least-32",
	})
	require.NoError(t, err)
	return service
}

func TestService_RegisterHashesPasswordAndPersistsUser(t *testing.T) {
	ctx := context.Background()
	input := &model.User{Username: "test-user", Email: "test@example.com"}
	passwords := &stubPasswords{hash: func(_ context.Context, password string) (string, error) {
		assert.Equal(t, "secret", password)
		return "encoded-hash", nil
	}}
	userService := &stubUsers{create: func(_ context.Context, got *model.User) (*model.User, error) {
		assert.Equal(t, "encoded-hash", got.PasswordHash)
		assert.Equal(t, input.Username, got.Username)
		return got, nil
	}}
	service := newService(t, &stubRepository{}, userService, passwords)

	created, err := service.Register(ctx, input, "secret")

	require.NoError(t, err)
	assert.Equal(t, "encoded-hash", created.PasswordHash)
	assert.Empty(t, input.PasswordHash, "registration should not mutate the supplied user")
}

func TestService_RegisterErrors(t *testing.T) {
	passwordErr := errors.New("hash failed")
	passwords := &stubPasswords{hash: func(context.Context, string) (string, error) { return "", passwordErr }}
	service := newService(t, &stubRepository{}, &stubUsers{}, passwords)

	_, err := service.Register(context.Background(), &model.User{}, "secret")
	assert.ErrorIs(t, err, passwordErr)
	_, err = service.Register(context.Background(), nil, "secret")
	assert.ErrorIs(t, err, auth.ErrInvalidCredentials)
}

func TestService_LoginIssuesTokensForActiveUser(t *testing.T) {
	ctx := context.Background()
	var storedID, storedUser string
	repo := &stubRepository{create: func(_ context.Context, tokenID, userID string, expiry time.Time) error {
		storedID, storedUser = tokenID, userID
		assert.True(t, expiry.After(time.Now()))
		return nil
	}}
	users := &stubUsers{get: func(_ context.Context, id string) (*model.User, error) {
		assert.Equal(t, "user-1", id)
		return &model.User{ID: id, PasswordHash: "hash", Status: model.UserStatusActive}, nil
	}}
	passwords := &stubPasswords{verify: func(_ context.Context, password, hash string) (bool, error) {
		assert.Equal(t, "secret", password)
		assert.Equal(t, "hash", hash)
		return true, nil
	}}
	service := newService(t, repo, users, passwords)

	pair, err := service.Login(ctx, "user-1", "secret")

	require.NoError(t, err)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)
	assert.Equal(t, pair.RefreshID, storedID)
	assert.Equal(t, "user-1", storedUser)
}

func TestService_LoginRejectsMissingUserBadPasswordAndInactiveUser(t *testing.T) {
	for _, tc := range []struct {
		name      string
		user      *model.User
		getErr    error
		valid     bool
		verifyErr error
		want      error
	}{
		{name: "missing user", getErr: errors.New("not found"), want: auth.ErrInvalidCredentials},
		{name: "wrong password", user: &model.User{ID: "u", PasswordHash: "hash", Status: model.UserStatusActive}, want: auth.ErrInvalidCredentials},
		{name: "verify error", user: &model.User{ID: "u", PasswordHash: "hash", Status: model.UserStatusActive}, verifyErr: errors.New("bad hash"), want: auth.ErrInvalidCredentials},
		{name: "inactive user", user: &model.User{ID: "u", PasswordHash: "hash", Status: model.UserStatusInvited}, valid: true, want: auth.ErrInactiveUser},
	} {
		t.Run(tc.name, func(t *testing.T) {
			users := &stubUsers{get: func(context.Context, string) (*model.User, error) { return tc.user, tc.getErr }}
			passwords := &stubPasswords{verify: func(context.Context, string, string) (bool, error) { return tc.valid, tc.verifyErr }}
			service := newService(t, &stubRepository{}, users, passwords)
			_, err := service.Login(context.Background(), "u", "password")
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestService_RefreshAndLogout(t *testing.T) {
	ctx := context.Background()
	manager, err := jwt.NewManager("access-secret-for-auth-tests-at-least-32", "refresh-secret-for-auth-tests-at-least-32")
	require.NoError(t, err)
	initial, err := manager.NewPair("user-1")
	require.NoError(t, err)
	var rotatedOldID, rotatedUser, revokedID string
	repo := &stubRepository{
		rotate: func(_ context.Context, oldID, userID, newID string, expires, now time.Time) error {
			rotatedOldID, rotatedUser = oldID, userID
			assert.NotEmpty(t, newID)
			assert.True(t, expires.After(now))
			return nil
		},
		revoke: func(_ context.Context, id, userID string, _ time.Time) error {
			revokedID = id
			assert.Equal(t, "user-1", userID)
			return nil
		},
	}
	service := newService(t, repo, &stubUsers{}, &stubPasswords{})

	rotated, err := service.Refresh(ctx, initial.RefreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, rotated.RefreshToken)
	assert.Equal(t, initial.RefreshID, rotatedOldID)
	assert.Equal(t, "user-1", rotatedUser)
	assert.NoError(t, service.Logout(ctx, rotated.RefreshToken))
	assert.Equal(t, rotated.RefreshID, revokedID)

	_, err = service.Refresh(ctx, "not-a-jwt")
	assert.ErrorIs(t, err, auth.ErrInvalidRefreshToken)
	assert.ErrorIs(t, service.Logout(ctx, "not-a-jwt"), auth.ErrInvalidRefreshToken)
}
