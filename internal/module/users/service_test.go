package users_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"tinyfold/internal/module/users"
	usersMock "tinyfold/internal/module/users/mock"
)

func newService(t *testing.T) (*usersMock.MockRepository, users.Service) {
	t.Helper()
	controller := gomock.NewController(t)
	repo := usersMock.NewMockRepository(controller)
	return repo, users.NewService(repo)
}

func TestService_Create(t *testing.T) {
	for _, tt := range []struct {
		name       string
		role       users.UserRole
		status     users.UserStatus
		wantRole   users.UserRole
		wantStatus users.UserStatus
	}{
		{name: "defaults empty role and status", wantRole: users.UserRoleAuthor, wantStatus: users.UserStatusInvited},
		{name: "accepts explicit author invited", role: users.UserRoleAuthor, status: users.UserStatusInvited, wantRole: users.UserRoleAuthor, wantStatus: users.UserStatusInvited},
		{name: "accepts editor invited", role: users.UserRoleEditor, status: users.UserStatusInvited, wantRole: users.UserRoleEditor, wantStatus: users.UserStatusInvited},
		{name: "accepts admin invited", role: users.UserRoleAdmin, status: users.UserStatusInvited, wantRole: users.UserRoleAdmin, wantStatus: users.UserStatusInvited},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo, service := newService(t)
			ctx := context.Background()
			input := &users.User{
				Username: "  jane-doe  ", Email: "  Jane@Example.COM  ",
				DisplayName: "  Jane Doe  ", Bio: "author bio",
				Role: tt.role, Status: tt.status,
			}
			original := cloneUser(*input)
			persisted := &users.User{
				ID: "user-1", Username: "jane-doe", Email: "jane@example.com",
				DisplayName: "Jane Doe", Bio: "author bio",
				Role: tt.wantRole, Status: tt.wantStatus,
				CreatedAt: time.Now().UTC(),
			}
			persisted.UpdatedAt = persisted.CreatedAt

			repo.EXPECT().Create(ctx, gomock.Any()).DoAndReturn(
				func(_ context.Context, got *users.User) (*users.User, error) {
					assert.Equal(t, "jane-doe", got.Username, "repository should receive a trimmed username")
					assert.Equal(t, "jane@example.com", got.Email, "repository should receive a trimmed lowercased email")
					assert.Equal(t, "Jane Doe", got.DisplayName, "repository should receive a trimmed display name")
					assert.Equal(t, "author bio", got.Bio, "repository should receive bio unchanged")
					assert.Equal(t, tt.wantRole, got.Role)
					assert.Equal(t, tt.wantStatus, got.Status)
					return persisted, nil
				},
			)

			got, err := service.Create(ctx, input)
			require.NoError(t, err)
			assert.Equal(t, persisted, got, "Create() should return the repository result")
			assert.Equal(t, original, *input, "Create() should not mutate input")
		})
	}
}

func TestService_CreateIgnoresClientManagedMetadata(t *testing.T) {
	repo, service := newService(t)
	ctx := context.Background()
	clientTime := time.Date(2040, 1, 2, 3, 4, 5, 0, time.UTC)
	input := &users.User{
		ID: "client-id", Username: "jane-doe", Email: "jane@example.com",
		DisplayName: "Jane", Role: users.UserRoleAuthor, Status: users.UserStatusInvited,
		CreatedAt: clientTime, UpdatedAt: clientTime,
	}
	original := cloneUser(*input)
	persisted := &users.User{
		ID: "generated-id", Username: "jane-doe", Email: "jane@example.com",
		DisplayName: "Jane", Role: users.UserRoleAuthor, Status: users.UserStatusInvited,
		CreatedAt: time.Now().UTC(),
	}
	persisted.UpdatedAt = persisted.CreatedAt

	repo.EXPECT().Create(ctx, gomock.Any()).DoAndReturn(
		func(_ context.Context, got *users.User) (*users.User, error) {
			assert.Empty(t, got.ID)
			assert.Zero(t, got.CreatedAt)
			assert.Zero(t, got.UpdatedAt)
			return persisted, nil
		},
	)

	got, err := service.Create(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, "generated-id", got.ID)
	assert.Equal(t, original, *input, "Create() should not mutate input")
}

func TestService_CreateValidation(t *testing.T) {
	valid := func() *users.User {
		return &users.User{
			Username: "jane-doe", Email: "jane@example.com",
			DisplayName: "Jane Doe", Role: users.UserRoleAuthor, Status: users.UserStatusInvited,
		}
	}
	tests := []struct {
		name string
		user func() *users.User
		field string
	}{
		{name: "nil user", user: func() *users.User { return nil }, field: "user"},
		{name: "blank username", user: func() *users.User { p := valid(); p.Username = " \t\n "; return p }, field: "username"},
		{name: "username too long", user: func() *users.User { p := valid(); p.Username = strings.Repeat("a", users.MaxUsernameLength+1); return p }, field: "username"},
		{name: "uppercase username", user: func() *users.User { p := valid(); p.Username = "Jane"; return p }, field: "username"},
		{name: "spaces in username", user: func() *users.User { p := valid(); p.Username = "jane doe"; return p }, field: "username"},
		{name: "leading hyphen", user: func() *users.User { p := valid(); p.Username = "-jane"; return p }, field: "username"},
		{name: "trailing hyphen", user: func() *users.User { p := valid(); p.Username = "jane-"; return p }, field: "username"},
		{name: "repeated hyphen", user: func() *users.User { p := valid(); p.Username = "jane--doe"; return p }, field: "username"},
		{name: "underscore in username", user: func() *users.User { p := valid(); p.Username = "jane_doe"; return p }, field: "username"},
		{name: "non-ascii username", user: func() *users.User { p := valid(); p.Username = "café"; return p }, field: "username"},
		{name: "blank email", user: func() *users.User { p := valid(); p.Email = "  "; return p }, field: "email"},
		{name: "email without at", user: func() *users.User { p := valid(); p.Email = "jane.example.com"; return p }, field: "email"},
		{name: "email without domain dot", user: func() *users.User { p := valid(); p.Email = "jane@localhost"; return p }, field: "email"},
		{name: "email with space", user: func() *users.User { p := valid(); p.Email = "jane @example.com"; return p }, field: "email"},
		{name: "email too long", user: func() *users.User { p := valid(); p.Email = strings.Repeat("a", users.MaxEmailLength) + "@example.com"; return p }, field: "email"},
		{name: "blank display name", user: func() *users.User { p := valid(); p.DisplayName = " \t\n "; return p }, field: "display_name"},
		{name: "display name too long", user: func() *users.User { p := valid(); p.DisplayName = strings.Repeat("x", users.MaxDisplayNameLength+1); return p }, field: "display_name"},
		{name: "bio too long", user: func() *users.User { p := valid(); p.Bio = strings.Repeat("x", users.MaxBioLength+1); return p }, field: "bio"},
		{name: "cannot create active user", user: func() *users.User { p := valid(); p.Status = users.UserStatusActive; return p }, field: "status"},
		{name: "cannot create disabled user", user: func() *users.User { p := valid(); p.Status = users.UserStatusDisabled; return p }, field: "status"},
		{name: "unknown status", user: func() *users.User { p := valid(); p.Status = "archived"; return p }, field: "status"},
		{name: "unknown role", user: func() *users.User { p := valid(); p.Role = "superadmin"; return p }, field: "role"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, service := newService(t)
			_, err := service.Create(context.Background(), tt.user())
			assertValidationError(t, err, tt.field)
		})
	}
}

func TestService_CreateAcceptsFieldBoundaries(t *testing.T) {
	for _, tt := range []struct {
		name string
		user *users.User
	}{
		{
			name: "username at max length",
			user: &users.User{Username: strings.Repeat("a", users.MaxUsernameLength), Email: "a@example.com", DisplayName: "Name"},
		},
		{
			name: "display name at max runes",
			user: &users.User{Username: "jane-doe", Email: "jane@example.com", DisplayName: strings.Repeat("é", users.MaxDisplayNameLength)},
		},
		{
			name: "bio at one mebibyte",
			user: &users.User{Username: "jane-doe", Email: "jane@example.com", DisplayName: "Jane", Bio: strings.Repeat("x", users.MaxBioLength)},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo, service := newService(t)
			repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(tt.user, nil)
			_, err := service.Create(context.Background(), tt.user)
			require.NoError(t, err, "Create() should accept allowed boundary")
		})
	}
}

func TestService_CreateReturnsRepositoryError(t *testing.T) {
	repo, service := newService(t)
	ctx := context.Background()
	repoErr := errors.New("duplicate email")
	repo.EXPECT().Create(ctx, gomock.Any()).Return(nil, repoErr)

	_, err := service.Create(ctx, &users.User{Username: "jane-doe", Email: "jane@example.com", DisplayName: "Jane"})
	require.ErrorIs(t, err, repoErr)
}

func TestService_GetMany(t *testing.T) {
	repo, service := newService(t)
	ctx := context.Background()
	want := []users.User{testUser("user-1", users.UserStatusInvited), testUser("user-2", users.UserStatusActive)}
	repo.EXPECT().GetMany(ctx).Return(want, nil)

	got, err := service.GetMany(ctx)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestService_GetManyNormalizesNilToEmpty(t *testing.T) {
	repo, service := newService(t)
	repo.EXPECT().GetMany(gomock.Any()).Return(nil, nil)

	got, err := service.GetMany(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, got, "GetMany() should return a non-nil empty slice")
	assert.Empty(t, got)
}

func TestService_GetManyReturnsRepositoryError(t *testing.T) {
	repo, service := newService(t)
	repoErr := errors.New("read failed")
	repo.EXPECT().GetMany(gomock.Any()).Return(nil, repoErr)

	_, err := service.GetMany(context.Background())
	require.ErrorIs(t, err, repoErr)
}

func TestService_GetByID(t *testing.T) {
	repo, service := newService(t)
	ctx := context.Background()
	want := testUser("user-1", users.UserStatusInvited)
	repo.EXPECT().GetByID(ctx, "user-1").Return(&want, nil)

	got, err := service.GetByID(ctx, " user-1 ")
	require.NoError(t, err)
	assert.Equal(t, &want, got)
}

func TestService_GetByIDRejectsBlankID(t *testing.T) {
	_, service := newService(t)
	_, err := service.GetByID(context.Background(), " \t ")
	assertValidationError(t, err, "id")
}

func TestService_GetByIDReturnsNotFound(t *testing.T) {
	repo, service := newService(t)
	repo.EXPECT().GetByID(gomock.Any(), "missing").Return(nil, users.ErrNotFound)

	_, err := service.GetByID(context.Background(), "missing")
	require.ErrorIs(t, err, users.ErrNotFound)
}

func TestService_GetByIDTreatsNilUserAsNotFound(t *testing.T) {
	repo, service := newService(t)
	repo.EXPECT().GetByID(gomock.Any(), "user-1").Return(nil, nil)

	got, err := service.GetByID(context.Background(), "user-1")
	assert.Nil(t, got)
	require.ErrorIs(t, err, users.ErrNotFound)
}

func TestService_GetByIDReturnsRepositoryError(t *testing.T) {
	repo, service := newService(t)
	repoErr := errors.New("lookup failed")
	repo.EXPECT().GetByID(gomock.Any(), "user-1").Return(nil, repoErr)

	_, err := service.GetByID(context.Background(), "user-1")
	require.ErrorIs(t, err, repoErr)
}

func TestService_UpdateActivatesInvitedAndPreservesManagedFields(t *testing.T) {
	repo, service := newService(t)
	ctx := context.Background()
	createdAt := time.Now().UTC().Add(-24 * time.Hour)
	current := users.User{
		ID: "user-1", Username: "jane-doe", Email: "jane@example.com",
		DisplayName: "Before", Bio: "old bio", Role: users.UserRoleAuthor, Status: users.UserStatusInvited,
		CreatedAt: createdAt, UpdatedAt: createdAt.Add(time.Hour),
	}
	clientTimestamp := time.Now().UTC().Add(24 * time.Hour)
	input := &users.User{
		ID: " user-1 ", Username: "  jane-doe-2  ", Email: "  JANE2@Example.COM  ",
		DisplayName: "  After  ", Bio: "new bio", Role: users.UserRoleEditor, Status: users.UserStatusActive,
		CreatedAt: clientTimestamp, UpdatedAt: clientTimestamp,
	}
	original := cloneUser(*input)
	var updated *users.User

	gomock.InOrder(
		repo.EXPECT().GetByID(ctx, "user-1").Return(&current, nil),
		repo.EXPECT().Update(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, got *users.User) (*users.User, error) {
				updated = cloneUserPtr(got)
				return cloneUserPtr(got), nil
			},
		),
	)
	start := time.Now().UTC()

	got, err := service.Update(ctx, input)
	end := time.Now().UTC()
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, updated)
	assert.Equal(t, current.ID, got.ID)
	assert.Equal(t, "jane-doe-2", got.Username)
	assert.Equal(t, "jane2@example.com", got.Email)
	assert.Equal(t, "After", got.DisplayName)
	assert.Equal(t, "new bio", got.Bio)
	assert.Equal(t, users.UserRoleEditor, got.Role)
	assert.Equal(t, users.UserStatusActive, got.Status)
	assert.Equal(t, createdAt, got.CreatedAt)
	assert.True(t, !got.UpdatedAt.Before(start) && !got.UpdatedAt.After(end), "UpdatedAt = %v, want a timestamp between %v and %v", got.UpdatedAt, start, end)
	assert.Equal(t, original, *input, "Update() should not mutate input")
}

func TestService_UpdateStatusTransitions(t *testing.T) {
	tests := []struct {
		name string
		from users.UserStatus
		to   users.UserStatus
	}{
		{name: "invited remains invited", from: users.UserStatusInvited, to: users.UserStatusInvited},
		{name: "invited becomes active", from: users.UserStatusInvited, to: users.UserStatusActive},
		{name: "invited becomes disabled", from: users.UserStatusInvited, to: users.UserStatusDisabled},
		{name: "active remains active", from: users.UserStatusActive, to: users.UserStatusActive},
		{name: "active becomes disabled", from: users.UserStatusActive, to: users.UserStatusDisabled},
		{name: "disabled remains disabled", from: users.UserStatusDisabled, to: users.UserStatusDisabled},
		{name: "disabled becomes active", from: users.UserStatusDisabled, to: users.UserStatusActive},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, service := newService(t)
			ctx := context.Background()
			current := testUser("user-1", tt.from)
			var updated *users.User
			gomock.InOrder(
				repo.EXPECT().GetByID(ctx, "user-1").Return(&current, nil),
				repo.EXPECT().Update(ctx, gomock.Any()).DoAndReturn(
					func(_ context.Context, got *users.User) (*users.User, error) {
						updated = cloneUserPtr(got)
						return cloneUserPtr(got), nil
					},
				),
			)
			start := time.Now().UTC()

			got, err := service.Update(ctx, &users.User{
				ID: current.ID, Username: "jane-doe", Email: "jane@example.com",
				DisplayName: "Updated", Bio: "bio", Role: users.UserRoleAuthor, Status: tt.to,
			})
			end := time.Now().UTC()
			require.NoError(t, err)
			require.NotNil(t, got)
			require.NotNil(t, updated)
			assert.Equal(t, updated, got, "Update() should return the repository result")
			assert.Equal(t, tt.to, got.Status)
			assert.True(t, !got.UpdatedAt.Before(start) && !got.UpdatedAt.After(end), "UpdatedAt = %v, want a timestamp between %v and %v", got.UpdatedAt, start, end)
		})
	}
}

func TestService_UpdateRejectsInvalidStatusTransitions(t *testing.T) {
	for _, tt := range []struct {
		from users.UserStatus
		to   users.UserStatus
	}{
		{from: users.UserStatusActive, to: users.UserStatusInvited},
		{from: users.UserStatusDisabled, to: users.UserStatusInvited},
	} {
		t.Run(string(tt.from)+"-to-"+string(tt.to), func(t *testing.T) {
			repo, service := newService(t)
			ctx := context.Background()
			current := testUser("user-1", tt.from)
			repo.EXPECT().GetByID(ctx, "user-1").Return(&current, nil)

			_, err := service.Update(ctx, &users.User{
				ID: "user-1", Username: "jane-doe", Email: "jane@example.com",
				DisplayName: "Jane", Role: users.UserRoleAuthor, Status: tt.to,
			})
			assertValidationError(t, err, "status")
		})
	}
}

func TestService_UpdateValidation(t *testing.T) {
	valid := func() *users.User {
		return &users.User{
			ID: "user-1", Username: "jane-doe", Email: "jane@example.com",
			DisplayName: "Jane Doe", Role: users.UserRoleAuthor, Status: users.UserStatusActive,
		}
	}
	tests := []struct {
		name string
		user func() *users.User
		field string
	}{
		{name: "nil user", user: func() *users.User { return nil }, field: "user"},
		{name: "blank ID", user: func() *users.User { p := valid(); p.ID = " \t "; return p }, field: "id"},
		{name: "blank username", user: func() *users.User { p := valid(); p.Username = "  "; return p }, field: "username"},
		{name: "invalid username", user: func() *users.User { p := valid(); p.Username = "not_a_name"; return p }, field: "username"},
		{name: "username too long", user: func() *users.User { p := valid(); p.Username = strings.Repeat("a", users.MaxUsernameLength+1); return p }, field: "username"},
		{name: "blank email", user: func() *users.User { p := valid(); p.Email = "  "; return p }, field: "email"},
		{name: "invalid email", user: func() *users.User { p := valid(); p.Email = "not-an-email"; return p }, field: "email"},
		{name: "email too long", user: func() *users.User { p := valid(); p.Email = strings.Repeat("a", users.MaxEmailLength) + "@example.com"; return p }, field: "email"},
		{name: "blank display name", user: func() *users.User { p := valid(); p.DisplayName = "  "; return p }, field: "display_name"},
		{name: "display name too long", user: func() *users.User { p := valid(); p.DisplayName = strings.Repeat("x", users.MaxDisplayNameLength+1); return p }, field: "display_name"},
		{name: "bio too long", user: func() *users.User { p := valid(); p.Bio = strings.Repeat("x", users.MaxBioLength+1); return p }, field: "bio"},
		{name: "role required", user: func() *users.User { p := valid(); p.Role = ""; return p }, field: "role"},
		{name: "unknown role", user: func() *users.User { p := valid(); p.Role = "superadmin"; return p }, field: "role"},
		{name: "status required", user: func() *users.User { p := valid(); p.Status = ""; return p }, field: "status"},
		{name: "unknown status", user: func() *users.User { p := valid(); p.Status = "archived"; return p }, field: "status"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, service := newService(t)
			_, err := service.Update(context.Background(), tt.user())
			assertValidationError(t, err, tt.field)
		})
	}
}

func TestService_UpdateReturnsNotFound(t *testing.T) {
	repo, service := newService(t)
	ctx := context.Background()
	repo.EXPECT().GetByID(ctx, "missing").Return(nil, users.ErrNotFound)

	_, err := service.Update(ctx, &users.User{
		ID: "missing", Username: "jane-doe", Email: "jane@example.com",
		DisplayName: "Jane", Role: users.UserRoleAuthor, Status: users.UserStatusActive,
	})
	require.ErrorIs(t, err, users.ErrNotFound)
}

func TestService_UpdateReturnsLookupError(t *testing.T) {
	repo, service := newService(t)
	ctx := context.Background()
	repoErr := errors.New("lookup failed")
	repo.EXPECT().GetByID(ctx, "user-1").Return(nil, repoErr)

	_, err := service.Update(ctx, &users.User{
		ID: "user-1", Username: "jane-doe", Email: "jane@example.com",
		DisplayName: "Jane", Role: users.UserRoleAuthor, Status: users.UserStatusActive,
	})
	require.ErrorIs(t, err, repoErr)
}

func TestService_UpdateReturnsRepositoryError(t *testing.T) {
	repo, service := newService(t)
	ctx := context.Background()
	current := testUser("user-1", users.UserStatusActive)
	repoErr := errors.New("update failed")
	gomock.InOrder(
		repo.EXPECT().GetByID(ctx, "user-1").Return(&current, nil),
		repo.EXPECT().Update(ctx, gomock.Any()).Return(nil, repoErr),
	)

	_, err := service.Update(ctx, &users.User{
		ID: "user-1", Username: "jane-doe", Email: "jane@example.com",
		DisplayName: "Jane", Role: users.UserRoleAuthor, Status: users.UserStatusActive,
	})
	require.ErrorIs(t, err, repoErr)
}

func TestService_Delete(t *testing.T) {
	repo, service := newService(t)
	ctx := context.Background()
	repo.EXPECT().Delete(ctx, "user-1").Return(nil)

	require.NoError(t, service.Delete(ctx, " user-1 "))
}

func TestService_DeleteRejectsBlankID(t *testing.T) {
	_, service := newService(t)
	assertValidationError(t, service.Delete(context.Background(), " \t "), "id")
}

func TestService_DeleteReturnsNotFound(t *testing.T) {
	repo, service := newService(t)
	repo.EXPECT().Delete(gomock.Any(), "missing").Return(users.ErrNotFound)

	require.ErrorIs(t, service.Delete(context.Background(), "missing"), users.ErrNotFound)
}

func TestService_DeleteReturnsRepositoryError(t *testing.T) {
	repo, service := newService(t)
	repoErr := errors.New("delete failed")
	repo.EXPECT().Delete(gomock.Any(), "user-1").Return(repoErr)

	require.ErrorIs(t, service.Delete(context.Background(), "user-1"), repoErr)
}

func assertValidationError(t *testing.T, err error, field string) {
	t.Helper()
	require.Error(t, err, "want invalid %s error", field)
	require.ErrorIs(t, err, users.ErrInvalidUser)
	var validationErr *users.ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Equal(t, field, validationErr.Field)
}

func testUser(id string, status users.UserStatus) users.User {
	now := time.Now().UTC().Add(-time.Hour)
	return users.User{
		ID: id, Username: "user-" + id, Email: "user-" + id + "@example.com",
		DisplayName: "Display " + id, Bio: "bio", Role: users.UserRoleAuthor, Status: status,
		CreatedAt: now, UpdatedAt: now,
	}
}

func cloneUser(user users.User) users.User {
	return user
}

func cloneUserPtr(user *users.User) *users.User {
	if user == nil {
		return nil
	}
	clone := cloneUser(*user)
	return &clone
}
