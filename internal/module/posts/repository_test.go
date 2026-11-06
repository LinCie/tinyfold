package posts_test

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
	"tinyfold/internal/model"
	"tinyfold/internal/module/posts"
)

func newSQLiteRepository(t *testing.T) (*sqlx.DB, posts.Repository) {
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

	return db, posts.NewSQLiteRepository(db)
}

func TestSQLiteRepository_Create(t *testing.T) {
	_, repo := newSQLiteRepository(t)
	ctx := context.Background()
	start := time.Now().UTC().Add(-time.Second)
	input := &model.Post{
		Title:   "O'Brien; --",
		Slug:    "hello-world",
		Content: "Post body",
		Status:  model.PostStatusDraft,
	}

	got, err := repo.Create(ctx, input)
	end := time.Now().UTC().Add(time.Second)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.NotEmpty(t, got.ID, "Create should assign an ID")
	assert.Equal(t, input.Title, got.Title)
	assert.Equal(t, input.Slug, got.Slug)
	assert.Equal(t, input.Content, got.Content)
	assert.Equal(t, input.Status, got.Status)
	assert.False(t, got.CreatedAt.Before(start), "CreatedAt = %v, want >= %v", got.CreatedAt, start)
	assert.False(t, got.CreatedAt.After(end), "CreatedAt = %v, want <= %v", got.CreatedAt, end)
	assert.True(t, got.CreatedAt.Equal(got.UpdatedAt), "new post should have matching creation and update times")
	assert.Nil(t, got.PublishedAt)
	assert.Nil(t, got.ArchivedAt)

	persisted, err := repo.GetByID(ctx, got.ID)
	require.NoError(t, err)
	require.NotNil(t, persisted)
	assertPostEqual(t, *got, *persisted)
}

func TestSQLiteRepository_CreateRejectsNilPost(t *testing.T) {
	_, repo := newSQLiteRepository(t)

	got, err := repo.Create(context.Background(), nil)

	assert.Nil(t, got)
	require.ErrorIs(t, err, posts.ErrInvalidPost)
}

func TestSQLiteRepository_CreateRejectsDatabaseConstraintViolations(t *testing.T) {
	for _, tt := range []struct {
		name   string
		mutate func(*model.Post)
	}{
		{
			name: "blank title",
			mutate: func(post *model.Post) {
				post.Title = ""
			},
		},
		{
			name: "blank slug",
			mutate: func(post *model.Post) {
				post.Slug = ""
			},
		},
		{
			name: "unknown status",
			mutate: func(post *model.Post) {
				post.Status = "scheduled"
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, repo := newSQLiteRepository(t)
			input := validDraftPost("valid-slug")
			tt.mutate(input)

			got, err := repo.Create(context.Background(), input)

			assert.Nil(t, got)
			requireSQLiteConstraintError(t, err)
		})
	}
}

func TestSQLiteRepository_CreateRejectsDuplicateSlug(t *testing.T) {
	db, repo := newSQLiteRepository(t)
	insertPost(t, db, postFixture("existing", "duplicate-slug", model.PostStatusDraft))

	got, err := repo.Create(context.Background(), validDraftPost("duplicate-slug"))

	assert.Nil(t, got)
	requireSQLiteConstraintError(t, err)
}

func TestSQLiteRepository_GetByID(t *testing.T) {
	db, repo := newSQLiteRepository(t)
	want := postFixture("post-1", "first-post", model.PostStatusPublished)
	publishedAt := time.Date(2025, time.January, 2, 3, 4, 5, 0, time.UTC)
	want.PublishedAt = &publishedAt
	insertPost(t, db, want)

	got, err := repo.GetByID(context.Background(), want.ID)

	require.NoError(t, err)
	require.NotNil(t, got)
	assertPostEqual(t, want, *got)
}

func TestSQLiteRepository_GetByIDNotFound(t *testing.T) {
	_, repo := newSQLiteRepository(t)

	got, err := repo.GetByID(context.Background(), "missing")

	assert.Nil(t, got)
	require.ErrorIs(t, err, posts.ErrNotFound)
}

func TestSQLiteRepository_GetMany(t *testing.T) {
	db, repo := newSQLiteRepository(t)
	first := postFixture("post-1", "first-post", model.PostStatusDraft)
	second := postFixture("post-2", "second-post", model.PostStatusArchived)
	archivedAt := time.Date(2025, time.February, 3, 4, 5, 6, 0, time.UTC)
	second.ArchivedAt = &archivedAt
	insertPost(t, db, first)
	insertPost(t, db, second)

	got, err := repo.GetMany(context.Background())

	require.NoError(t, err)
	require.Len(t, got, 2)
	byID := make(map[string]model.Post, len(got))
	for _, post := range got {
		byID[post.ID] = post
	}
	require.Contains(t, byID, first.ID)
	require.Contains(t, byID, second.ID)
	assertPostEqual(t, first, byID[first.ID])
	assertPostEqual(t, second, byID[second.ID])
}

func TestSQLiteRepository_GetManyEmpty(t *testing.T) {
	_, repo := newSQLiteRepository(t)

	got, err := repo.GetMany(context.Background())

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestSQLiteRepository_Update(t *testing.T) {
	db, repo := newSQLiteRepository(t)
	original := postFixture("post-1", "first-post", model.PostStatusDraft)
	insertPost(t, db, original)

	publishedAt := time.Date(2025, time.March, 4, 5, 6, 7, 0, time.UTC)
	updated := model.Post{
		ID:          original.ID,
		Title:       "Updated title",
		Slug:        "updated-post",
		Content:     "Updated content",
		Status:      model.PostStatusPublished,
		CreatedAt:   original.CreatedAt,
		UpdatedAt:   time.Date(2025, time.March, 5, 6, 7, 8, 0, time.UTC),
		PublishedAt: &publishedAt,
	}

	got, err := repo.Update(context.Background(), &updated)

	require.NoError(t, err)
	require.NotNil(t, got)
	assertPostEqual(t, updated, *got)

	persisted, err := repo.GetByID(context.Background(), updated.ID)
	require.NoError(t, err)
	require.NotNil(t, persisted)
	assertPostEqual(t, updated, *persisted)
}

func TestSQLiteRepository_UpdateNotFound(t *testing.T) {
	_, repo := newSQLiteRepository(t)
	missing := postFixture("missing", "missing-post", model.PostStatusDraft)

	got, err := repo.Update(context.Background(), &missing)

	assert.Nil(t, got)
	require.ErrorIs(t, err, posts.ErrNotFound)
}

func TestSQLiteRepository_UpdateRejectsNilPost(t *testing.T) {
	_, repo := newSQLiteRepository(t)

	got, err := repo.Update(context.Background(), nil)

	assert.Nil(t, got)
	require.ErrorIs(t, err, posts.ErrInvalidPost)
}

func TestSQLiteRepository_Delete(t *testing.T) {
	db, repo := newSQLiteRepository(t)
	post := postFixture("post-1", "first-post", model.PostStatusDraft)
	insertPost(t, db, post)

	err := repo.Delete(context.Background(), post.ID)
	require.NoError(t, err)

	got, err := repo.GetByID(context.Background(), post.ID)
	assert.Nil(t, got)
	require.ErrorIs(t, err, posts.ErrNotFound)
}

func TestSQLiteRepository_PropagatesCanceledContext(t *testing.T) {
	_, repo := newSQLiteRepository(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := repo.GetByID(ctx, "post-1")

	require.ErrorIs(t, err, context.Canceled)
}

func validDraftPost(slug string) *model.Post {
	return &model.Post{
		Title:   "A title",
		Slug:    slug,
		Content: "Post body",
		Status:  model.PostStatusDraft,
	}
}

func postFixture(id, slug string, status model.PostStatus) model.Post {
	return model.Post{
		ID:        id,
		Title:     "Title for " + id,
		Slug:      slug,
		Content:   "Content for " + id,
		Status:    status,
		CreatedAt: time.Date(2025, time.January, 1, 2, 3, 4, 0, time.UTC),
		UpdatedAt: time.Date(2025, time.January, 1, 2, 3, 4, 0, time.UTC),
	}
}

func insertPost(t *testing.T, db *sqlx.DB, post model.Post) {
	t.Helper()

	_, err := db.ExecContext(
		context.Background(),
		`INSERT INTO posts (
			id, title, slug, content, status, created_at, updated_at, published_at, archived_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		post.ID,
		post.Title,
		post.Slug,
		post.Content,
		post.Status,
		post.CreatedAt,
		post.UpdatedAt,
		nullableTime(post.PublishedAt),
		nullableTime(post.ArchivedAt),
	)
	require.NoError(t, err)
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}

func requireSQLiteConstraintError(t *testing.T, err error) {
	t.Helper()

	var sqliteErr *sqlite.Error
	require.ErrorAs(t, err, &sqliteErr)
	assert.Equal(t, sqlite3.SQLITE_CONSTRAINT, sqliteErr.Code()&0xff)
}

func assertPostEqual(t *testing.T, want, got model.Post) {
	t.Helper()

	assert.Equal(t, want.ID, got.ID)
	assert.Equal(t, want.Title, got.Title)
	assert.Equal(t, want.Slug, got.Slug)
	assert.Equal(t, want.Content, got.Content)
	assert.Equal(t, want.Status, got.Status)
	assert.True(t, want.CreatedAt.Equal(got.CreatedAt), "CreatedAt = %v, want %v", got.CreatedAt, want.CreatedAt)
	assert.True(t, want.UpdatedAt.Equal(got.UpdatedAt), "UpdatedAt = %v, want %v", got.UpdatedAt, want.UpdatedAt)
	assertOptionalTimeEqual(t, want.PublishedAt, got.PublishedAt, "PublishedAt")
	assertOptionalTimeEqual(t, want.ArchivedAt, got.ArchivedAt, "ArchivedAt")
}

func assertOptionalTimeEqual(t *testing.T, want, got *time.Time, field string) {
	t.Helper()

	if want == nil {
		assert.Nil(t, got, "%s should be nil", field)
		return
	}
	require.NotNil(t, got, "%s should not be nil", field)
	assert.True(t, want.Equal(*got), "%s = %v, want %v", field, *got, *want)
}
