package users_test

import (
	"context"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"tinyfold/internal/database"
	"tinyfold/internal/module/users"
)

func newSQLiteRepository(t *testing.T) (*sqlx.DB, users.Repository) {
	t.Helper()

	db, err := database.NewDatabase(":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)

	require.NoError(t, goose.SetDialect("sqlite"))
	err = goose.UpContext(context.Background(), db.DB, "../../../migrations")
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})

	return db, users.NewSQLiteRepository(db)
}

func TestSQLiteRepository_Create(t *testing.T) {
	_, repo := newSQLiteRepository(t)
	ctx := context.Background()
	start := time.Now().UTC().Add(-time.Second)
	input := &users.User{
		Username:    "jane-doe",
		Email:       "jane@example.com",
		DisplayName: "Jane O'Brien; --",
		Bio:         "CMS author bio",
		Role:        users.UserRoleAuthor,
		Status:      users.UserStatusInvited,
	}

	got, err := repo.Create(ctx, input)
	end := time.Now().UTC().Add(time.Second)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.NotEmpty(t, got.ID, "Create should assign an ID")
	assert.Equal(t, input.Username, got.Username)
	assert.Equal(t, input.Email, got.Email)
	assert.Equal(t, input.DisplayName, got.DisplayName)
	assert.Equal(t, input.Bio, got.Bio)
	assert.Equal(t, input.Role, got.Role)
	assert.Equal(t, input.Status, got.Status)
	assert.False(t, got.CreatedAt.Before(start), "CreatedAt = %v, want >= %v", got.CreatedAt, start)
	assert.False(t, got.CreatedAt.After(end), "CreatedAt = %v, want <= %v", got.CreatedAt, end)
	assert.True(t, got.CreatedAt.Equal(got.UpdatedAt), "new user should have matching creation and update times")

	persisted, err := repo.GetByID(ctx, got.ID)
	require.NoError(t, err)
	require.NotNil(t, persisted)
	assertUserEqual(t, *got, *persisted)
}

func TestSQLiteRepository_CreateRejectsNilUser(t *testing.T) {
	_, repo := newSQLiteRepository(t)

	got, err := repo.Create(context.Background(), nil)

	assert.Nil(t, got)
	require.ErrorIs(t, err, users.ErrInvalidUser)
}

func TestSQLiteRepository_CreateRejectsDatabaseConstraintViolations(t *testing.T) {
	for _, tt := range []struct {
		name   string
		mutate func(*users.User)
	}{
		{
			name: "blank username",
			mutate: func(user *users.User) {
				user.Username = ""
			},
		},
		{
			name: "blank email",
			mutate: func(user *users.User) {
				user.Email = ""
			},
		},
		{
			name: "blank display name",
			mutate: func(user *users.User) {
				user.DisplayName = ""
			},
		},
		{
			name: "unknown role",
			mutate: func(user *users.User) {
				user.Role = "superadmin"
			},
		},
		{
			name: "unknown status",
			mutate: func(user *users.User) {
				user.Status = "archived"
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, repo := newSQLiteRepository(t)
			input := validInvitedUser("valid-user")
			tt.mutate(input)

			got, err := repo.Create(context.Background(), input)

			assert.Nil(t, got)
			requireSQLiteConstraintError(t, err)
		})
	}
}

func TestSQLiteRepository_CreateRejectsDuplicateUsername(t *testing.T) {
	db, repo := newSQLiteRepository(t)
	insertUser(t, db, userFixture("existing", "duplicate-user", users.UserRoleAuthor, users.UserStatusInvited))

	got, err := repo.Create(context.Background(), validInvitedUser("duplicate-user"))

	assert.Nil(t, got)
	requireSQLiteConstraintError(t, err)
}

func TestSQLiteRepository_CreateRejectsDuplicateEmail(t *testing.T) {
	db, repo := newSQLiteRepository(t)
	existing := userFixture("existing", "existing-user", users.UserRoleAuthor, users.UserStatusInvited)
	insertUser(t, db, existing)

	input := validInvitedUser("another-user")
	input.Email = existing.Email

	got, err := repo.Create(context.Background(), input)

	assert.Nil(t, got)
	requireSQLiteConstraintError(t, err)
}

func TestSQLiteRepository_GetByID(t *testing.T) {
	db, repo := newSQLiteRepository(t)
	want := userFixture("user-1", "first-user", users.UserRoleEditor, users.UserStatusActive)
	insertUser(t, db, want)

	got, err := repo.GetByID(context.Background(), want.ID)

	require.NoError(t, err)
	require.NotNil(t, got)
	assertUserEqual(t, want, *got)
}

func TestSQLiteRepository_GetByIDNotFound(t *testing.T) {
	_, repo := newSQLiteRepository(t)

	got, err := repo.GetByID(context.Background(), "missing")

	assert.Nil(t, got)
	require.ErrorIs(t, err, users.ErrNotFound)
}

func TestSQLiteRepository_GetMany(t *testing.T) {
	db, repo := newSQLiteRepository(t)
	first := userFixture("user-1", "first-user", users.UserRoleAuthor, users.UserStatusInvited)
	second := userFixture("user-2", "second-user", users.UserRoleAdmin, users.UserStatusDisabled)
	insertUser(t, db, first)
	insertUser(t, db, second)

	got, err := repo.GetMany(context.Background())

	require.NoError(t, err)
	require.Len(t, got, 2)
	byID := make(map[string]users.User, len(got))
	for _, user := range got {
		byID[user.ID] = user
	}
	require.Contains(t, byID, first.ID)
	require.Contains(t, byID, second.ID)
	assertUserEqual(t, first, byID[first.ID])
	assertUserEqual(t, second, byID[second.ID])
}

func TestSQLiteRepository_GetManyEmpty(t *testing.T) {
	_, repo := newSQLiteRepository(t)

	got, err := repo.GetMany(context.Background())

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestSQLiteRepository_Update(t *testing.T) {
	db, repo := newSQLiteRepository(t)
	original := userFixture("user-1", "first-user", users.UserRoleAuthor, users.UserStatusInvited)
	insertUser(t, db, original)

	updated := users.User{
		ID:          original.ID,
		Username:    "updated-user",
		Email:       "updated@example.com",
		DisplayName: "Updated Name",
		Bio:         "Updated bio",
		Role:        users.UserRoleEditor,
		Status:      users.UserStatusActive,
		CreatedAt:   original.CreatedAt,
		UpdatedAt:   time.Date(2025, time.March, 5, 6, 7, 8, 0, time.UTC),
	}

	got, err := repo.Update(context.Background(), &updated)

	require.NoError(t, err)
	require.NotNil(t, got)
	assertUserEqual(t, updated, *got)

	persisted, err := repo.GetByID(context.Background(), updated.ID)
	require.NoError(t, err)
	require.NotNil(t, persisted)
	assertUserEqual(t, updated, *persisted)
}

func TestSQLiteRepository_UpdateNotFound(t *testing.T) {
	_, repo := newSQLiteRepository(t)
	missing := userFixture("missing", "missing-user", users.UserRoleAuthor, users.UserStatusInvited)

	got, err := repo.Update(context.Background(), &missing)

	assert.Nil(t, got)
	require.ErrorIs(t, err, users.ErrNotFound)
}

func TestSQLiteRepository_UpdateRejectsNilUser(t *testing.T) {
	_, repo := newSQLiteRepository(t)

	got, err := repo.Update(context.Background(), nil)

	assert.Nil(t, got)
	require.ErrorIs(t, err, users.ErrInvalidUser)
}

func TestSQLiteRepository_Delete(t *testing.T) {
	db, repo := newSQLiteRepository(t)
	user := userFixture("user-1", "first-user", users.UserRoleAuthor, users.UserStatusInvited)
	insertUser(t, db, user)

	err := repo.Delete(context.Background(), user.ID)
	require.NoError(t, err)

	got, err := repo.GetByID(context.Background(), user.ID)
	assert.Nil(t, got)
	require.ErrorIs(t, err, users.ErrNotFound)
}

func TestSQLiteRepository_PropagatesCanceledContext(t *testing.T) {
	_, repo := newSQLiteRepository(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := repo.GetByID(ctx, "user-1")

	require.ErrorIs(t, err, context.Canceled)
}

func validInvitedUser(username string) *users.User {
	return &users.User{
		Username:    username,
		Email:       username + "@example.com",
		DisplayName: "Display " + username,
		Bio:         "Bio for " + username,
		Role:        users.UserRoleAuthor,
		Status:      users.UserStatusInvited,
	}
}

func userFixture(id, username string, role users.UserRole, status users.UserStatus) users.User {
	return users.User{
		ID:          id,
		Username:    username,
		Email:       username + "@example.com",
		DisplayName: "Display " + id,
		Bio:         "Bio for " + id,
		Role:        role,
		Status:      status,
		CreatedAt:   time.Date(2025, time.January, 1, 2, 3, 4, 0, time.UTC),
		UpdatedAt:   time.Date(2025, time.January, 1, 2, 3, 4, 0, time.UTC),
	}
}

func insertUser(t *testing.T, db *sqlx.DB, user users.User) {
	t.Helper()

	_, err := db.ExecContext(
		context.Background(),
		`INSERT INTO users (
			id, username, email, display_name, bio, role, status, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		user.ID,
		user.Username,
		user.Email,
		user.DisplayName,
		user.Bio,
		user.Role,
		user.Status,
		user.CreatedAt,
		user.UpdatedAt,
	)
	require.NoError(t, err)
}

func requireSQLiteConstraintError(t *testing.T, err error) {
	t.Helper()

	var sqliteErr *sqlite.Error
	require.ErrorAs(t, err, &sqliteErr)
	assert.Equal(t, sqlite3.SQLITE_CONSTRAINT, sqliteErr.Code()&0xff)
}

func assertUserEqual(t *testing.T, want, got users.User) {
	t.Helper()

	assert.Equal(t, want.ID, got.ID)
	assert.Equal(t, want.Username, got.Username)
	assert.Equal(t, want.Email, got.Email)
	assert.Equal(t, want.DisplayName, got.DisplayName)
	assert.Equal(t, want.Bio, got.Bio)
	assert.Equal(t, want.Role, got.Role)
	assert.Equal(t, want.Status, got.Status)
	assert.True(t, want.CreatedAt.Equal(got.CreatedAt), "CreatedAt = %v, want %v", got.CreatedAt, want.CreatedAt)
	assert.True(t, want.UpdatedAt.Equal(got.UpdatedAt), "UpdatedAt = %v, want %v", got.UpdatedAt, want.UpdatedAt)
}
