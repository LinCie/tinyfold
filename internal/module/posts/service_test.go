package posts_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"tinyfold/internal/module/posts"
	postsMock "tinyfold/internal/module/posts/mock"
)

func newService(t *testing.T) (*postsMock.MockRepository, posts.Service) {
	t.Helper()
	controller := gomock.NewController(t)
	repo := postsMock.NewMockRepository(controller)
	return repo, posts.NewService(repo)
}

func TestService_Create(t *testing.T) {
	for _, tt := range []struct {
		name       string
		status     posts.PostStatus
		wantStatus posts.PostStatus
	}{
		{name: "defaults empty status to draft", wantStatus: posts.PostStatusDraft},
		{name: "accepts explicit draft", status: posts.PostStatusDraft, wantStatus: posts.PostStatusDraft},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo, service := newService(t)
			ctx := context.Background()
			input := &posts.Post{
				Title: "  Hello world  ", Slug: " hello-world ", Content: "", Status: tt.status,
			}
			original := clonePost(*input)
			persisted := &posts.Post{
				ID: "post-1", Title: "Hello world", Slug: "hello-world", Status: tt.wantStatus,
				CreatedAt: time.Now().UTC(),
			}
			persisted.UpdatedAt = persisted.CreatedAt

			repo.EXPECT().Create(ctx, gomock.Any()).DoAndReturn(
				func(_ context.Context, got *posts.Post) (*posts.Post, error) {
					assert.Equal(t, "Hello world", got.Title, "repository should receive a trimmed title")
					assert.Equal(t, "hello-world", got.Slug, "repository should receive a trimmed slug")
					assert.Empty(t, got.Content, "repository should receive content unchanged")
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
	input := &posts.Post{
		ID: "client-id", Title: "Title", Slug: "title", Status: posts.PostStatusDraft,
		CreatedAt: clientTime, UpdatedAt: clientTime, PublishedAt: &clientTime, ArchivedAt: &clientTime,
	}
	original := clonePost(*input)
	persisted := &posts.Post{
		ID: "generated-id", Title: "Title", Slug: "title", Status: posts.PostStatusDraft,
		CreatedAt: time.Now().UTC(),
	}
	persisted.UpdatedAt = persisted.CreatedAt

	repo.EXPECT().Create(ctx, gomock.Any()).DoAndReturn(
		func(_ context.Context, got *posts.Post) (*posts.Post, error) {
			assert.Empty(t, got.ID)
			assert.Zero(t, got.CreatedAt)
			assert.Zero(t, got.UpdatedAt)
			assert.Nil(t, got.PublishedAt)
			assert.Nil(t, got.ArchivedAt)
			return persisted, nil
		},
	)

	got, err := service.Create(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, "generated-id", got.ID)
	assert.Nil(t, got.PublishedAt)
	assert.Nil(t, got.ArchivedAt)
	assert.Equal(t, original, *input, "Create() should not mutate input")
}

func TestService_CreateValidation(t *testing.T) {
	valid := func() *posts.Post {
		return &posts.Post{Title: "Title", Slug: "title", Status: posts.PostStatusDraft}
	}
	tests := []struct {
		name  string
		post  func() *posts.Post
		field string
	}{
		{name: "nil post", post: func() *posts.Post { return nil }, field: "post"},
		{name: "blank title", post: func() *posts.Post { p := valid(); p.Title = " \t\n "; return p }, field: "title"},
		{name: "title too long", post: func() *posts.Post { p := valid(); p.Title = strings.Repeat("x", posts.MaxTitleLength+1); return p }, field: "title"},
		{name: "blank slug", post: func() *posts.Post { p := valid(); p.Slug = "  "; return p }, field: "slug"},
		{name: "uppercase slug", post: func() *posts.Post { p := valid(); p.Slug = "Hello"; return p }, field: "slug"},
		{name: "spaces in slug", post: func() *posts.Post { p := valid(); p.Slug = "hello world"; return p }, field: "slug"},
		{name: "leading hyphen", post: func() *posts.Post { p := valid(); p.Slug = "-hello"; return p }, field: "slug"},
		{name: "trailing hyphen", post: func() *posts.Post { p := valid(); p.Slug = "hello-"; return p }, field: "slug"},
		{name: "repeated hyphen", post: func() *posts.Post { p := valid(); p.Slug = "hello--world"; return p }, field: "slug"},
		{name: "underscore in slug", post: func() *posts.Post { p := valid(); p.Slug = "hello_world"; return p }, field: "slug"},
		{name: "non-ascii slug", post: func() *posts.Post { p := valid(); p.Slug = "café"; return p }, field: "slug"},
		{name: "slug too long", post: func() *posts.Post { p := valid(); p.Slug = strings.Repeat("a", posts.MaxSlugLength+1); return p }, field: "slug"},
		{name: "content too long", post: func() *posts.Post { p := valid(); p.Content = strings.Repeat("x", posts.MaxContentLength+1); return p }, field: "content"},
		{name: "cannot create published post", post: func() *posts.Post { p := valid(); p.Status = posts.PostStatusPublished; return p }, field: "status"},
		{name: "cannot create archived post", post: func() *posts.Post { p := valid(); p.Status = posts.PostStatusArchived; return p }, field: "status"},
		{name: "unknown status", post: func() *posts.Post { p := valid(); p.Status = "scheduled"; return p }, field: "status"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, service := newService(t)
			_, err := service.Create(context.Background(), tt.post())
			assertValidationError(t, err, tt.field)
		})
	}
}

func TestService_CreateAcceptsFieldBoundaries(t *testing.T) {
	for _, tt := range []struct {
		name string
		post *posts.Post
	}{
		{
			name: "200 character title and slug",
			post: &posts.Post{Title: strings.Repeat("é", posts.MaxTitleLength), Slug: strings.Repeat("a", posts.MaxSlugLength)},
		},
		{
			name: "content at one mebibyte",
			post: &posts.Post{Title: "Title", Slug: "title", Content: strings.Repeat("x", posts.MaxContentLength)},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo, service := newService(t)
			repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(tt.post, nil)
			_, err := service.Create(context.Background(), tt.post)
			require.NoError(t, err, "Create() should accept allowed boundary")
		})
	}
}

func TestService_CreateReturnsRepositoryError(t *testing.T) {
	repo, service := newService(t)
	ctx := context.Background()
	repoErr := errors.New("duplicate slug")
	repo.EXPECT().Create(ctx, gomock.Any()).Return(nil, repoErr)

	_, err := service.Create(ctx, &posts.Post{Title: "Title", Slug: "title"})
	require.ErrorIs(t, err, repoErr)
}

func TestService_GetMany(t *testing.T) {
	repo, service := newService(t)
	ctx := context.Background()
	want := []posts.Post{testPost("first", posts.PostStatusDraft), testPost("second", posts.PostStatusPublished)}
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
	want := testPost("post-1", posts.PostStatusDraft)
	repo.EXPECT().GetByID(ctx, "post-1").Return(&want, nil)

	got, err := service.GetByID(ctx, " post-1 ")
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
	repo.EXPECT().GetByID(gomock.Any(), "missing").Return(nil, posts.ErrNotFound)

	_, err := service.GetByID(context.Background(), "missing")
	require.ErrorIs(t, err, posts.ErrNotFound)
}

func TestService_GetByIDTreatsNilPostAsNotFound(t *testing.T) {
	repo, service := newService(t)
	repo.EXPECT().GetByID(gomock.Any(), "post-1").Return(nil, nil)

	got, err := service.GetByID(context.Background(), "post-1")
	assert.Nil(t, got)
	require.ErrorIs(t, err, posts.ErrNotFound)
}

func TestService_GetByIDReturnsRepositoryError(t *testing.T) {
	repo, service := newService(t)
	repoErr := errors.New("lookup failed")
	repo.EXPECT().GetByID(gomock.Any(), "post-1").Return(nil, repoErr)

	_, err := service.GetByID(context.Background(), "post-1")
	require.ErrorIs(t, err, repoErr)
}

func TestService_UpdatePublishesDraftAndPreservesManagedFields(t *testing.T) {
	repo, service := newService(t)
	ctx := context.Background()
	createdAt := time.Now().UTC().Add(-24 * time.Hour)
	current := posts.Post{
		ID: "post-1", Title: "Before", Slug: "before", Content: "old content", Status: posts.PostStatusDraft,
		CreatedAt: createdAt, UpdatedAt: createdAt.Add(time.Hour),
	}
	clientTimestamp := time.Now().UTC().Add(24 * time.Hour)
	input := &posts.Post{
		ID: " post-1 ", Title: "  After  ", Slug: " after ", Content: "new content", Status: posts.PostStatusPublished,
		CreatedAt: clientTimestamp, UpdatedAt: clientTimestamp, PublishedAt: &clientTimestamp, ArchivedAt: &clientTimestamp,
	}
	original := clonePost(*input)
	var updated *posts.Post

	gomock.InOrder(
		repo.EXPECT().GetByID(ctx, "post-1").Return(&current, nil),
		repo.EXPECT().Update(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, got *posts.Post) (*posts.Post, error) {
				updated = clonePostPtr(got)
				return clonePostPtr(got), nil
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
	assert.Equal(t, "After", got.Title)
	assert.Equal(t, "after", got.Slug)
	assert.Equal(t, "new content", got.Content)
	assert.Equal(t, posts.PostStatusPublished, got.Status)
	assert.Equal(t, createdAt, got.CreatedAt)
	assert.True(t, !got.UpdatedAt.Before(start) && !got.UpdatedAt.After(end), "UpdatedAt = %v, want a timestamp between %v and %v", got.UpdatedAt, start, end)
	require.NotNil(t, got.PublishedAt)
	assert.True(t, !got.PublishedAt.Before(start) && !got.PublishedAt.After(end), "PublishedAt = %v, want a timestamp between %v and %v", got.PublishedAt, start, end)
	assert.Nil(t, got.ArchivedAt, "publishing should not set ArchivedAt")
	assert.Equal(t, original, *input, "Update() should not mutate input")
}

func TestService_UpdateStatusTransitionsAndTimestamps(t *testing.T) {
	oldTime := time.Now().UTC().Add(-48 * time.Hour)
	oldPublishedAt := oldTime.Add(time.Hour)
	oldArchivedAt := oldTime.Add(2 * time.Hour)
	tests := []struct {
		name             string
		from             posts.PostStatus
		to               posts.PostStatus
		publishedAt      *time.Time
		archivedAt       *time.Time
		wantPublishedSet bool
		wantArchivedSet  bool
		preservePub      bool
		preserveArchived bool
	}{
		{name: "draft remains draft", from: posts.PostStatusDraft, to: posts.PostStatusDraft},
		{name: "draft becomes published", from: posts.PostStatusDraft, to: posts.PostStatusPublished, wantPublishedSet: true},
		{name: "draft becomes archived", from: posts.PostStatusDraft, to: posts.PostStatusArchived, wantArchivedSet: true},
		{name: "published remains published", from: posts.PostStatusPublished, to: posts.PostStatusPublished, publishedAt: &oldPublishedAt, wantPublishedSet: true, preservePub: true},
		{name: "published becomes archived", from: posts.PostStatusPublished, to: posts.PostStatusArchived, publishedAt: &oldPublishedAt, wantPublishedSet: true, wantArchivedSet: true, preservePub: true},
		{name: "archived remains archived", from: posts.PostStatusArchived, to: posts.PostStatusArchived, publishedAt: &oldPublishedAt, archivedAt: &oldArchivedAt, wantPublishedSet: true, wantArchivedSet: true, preservePub: true, preserveArchived: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, service := newService(t)
			ctx := context.Background()
			current := testPost("post-1", tt.from)
			current.CreatedAt = oldTime
			current.UpdatedAt = oldTime
			current.PublishedAt = cloneTime(tt.publishedAt)
			current.ArchivedAt = cloneTime(tt.archivedAt)
			var updated *posts.Post
			gomock.InOrder(
				repo.EXPECT().GetByID(ctx, "post-1").Return(&current, nil),
				repo.EXPECT().Update(ctx, gomock.Any()).DoAndReturn(
					func(_ context.Context, got *posts.Post) (*posts.Post, error) {
						updated = clonePostPtr(got)
						return clonePostPtr(got), nil
					},
				),
			)
			start := time.Now().UTC()

			got, err := service.Update(ctx, &posts.Post{
				ID: current.ID, Title: "Updated", Slug: "updated", Content: "content", Status: tt.to,
			})
			end := time.Now().UTC()
			require.NoError(t, err)
			require.NotNil(t, got)
			require.NotNil(t, updated)
			assert.Equal(t, updated, got, "Update() should return the repository result")
			assert.Equal(t, tt.to, got.Status)
			assert.True(t, !got.UpdatedAt.Before(start) && !got.UpdatedAt.After(end), "UpdatedAt = %v, want a timestamp between %v and %v", got.UpdatedAt, start, end)
			if tt.wantPublishedSet {
				assert.NotNil(t, got.PublishedAt, "Update() should set PublishedAt")
			} else {
				assert.Nil(t, got.PublishedAt, "Update() should not set PublishedAt")
			}
			if tt.wantArchivedSet {
				assert.NotNil(t, got.ArchivedAt, "Update() should set ArchivedAt")
			} else {
				assert.Nil(t, got.ArchivedAt, "Update() should not set ArchivedAt")
			}
			if tt.preservePub {
				assert.Equal(t, tt.publishedAt, got.PublishedAt, "Update() should preserve PublishedAt")
			}
			if tt.preserveArchived {
				assert.Equal(t, tt.archivedAt, got.ArchivedAt, "Update() should preserve ArchivedAt")
			}
		})
	}
}

func TestService_UpdateRejectsInvalidStatusTransitions(t *testing.T) {
	for _, tt := range []struct {
		from posts.PostStatus
		to   posts.PostStatus
	}{
		{from: posts.PostStatusPublished, to: posts.PostStatusDraft},
		{from: posts.PostStatusArchived, to: posts.PostStatusDraft},
		{from: posts.PostStatusArchived, to: posts.PostStatusPublished},
	} {
		t.Run(string(tt.from)+"-to-"+string(tt.to), func(t *testing.T) {
			repo, service := newService(t)
			ctx := context.Background()
			current := testPost("post-1", tt.from)
			repo.EXPECT().GetByID(ctx, "post-1").Return(&current, nil)

			_, err := service.Update(ctx, &posts.Post{
				ID: "post-1", Title: "Title", Slug: "title", Status: tt.to,
			})
			assertValidationError(t, err, "status")
		})
	}
}

func TestService_UpdateValidation(t *testing.T) {
	valid := func() *posts.Post {
		return &posts.Post{ID: "post-1", Title: "Title", Slug: "title", Status: posts.PostStatusDraft}
	}
	tests := []struct {
		name  string
		post  func() *posts.Post
		field string
	}{
		{name: "nil post", post: func() *posts.Post { return nil }, field: "post"},
		{name: "blank ID", post: func() *posts.Post { p := valid(); p.ID = " \t "; return p }, field: "id"},
		{name: "blank title", post: func() *posts.Post { p := valid(); p.Title = "  "; return p }, field: "title"},
		{name: "title too long", post: func() *posts.Post { p := valid(); p.Title = strings.Repeat("x", posts.MaxTitleLength+1); return p }, field: "title"},
		{name: "blank slug", post: func() *posts.Post { p := valid(); p.Slug = "  "; return p }, field: "slug"},
		{name: "invalid slug", post: func() *posts.Post { p := valid(); p.Slug = "not_a_slug"; return p }, field: "slug"},
		{name: "slug too long", post: func() *posts.Post { p := valid(); p.Slug = strings.Repeat("a", posts.MaxSlugLength+1); return p }, field: "slug"},
		{name: "content too long", post: func() *posts.Post { p := valid(); p.Content = strings.Repeat("x", posts.MaxContentLength+1); return p }, field: "content"},
		{name: "status required", post: func() *posts.Post { p := valid(); p.Status = ""; return p }, field: "status"},
		{name: "unknown status", post: func() *posts.Post { p := valid(); p.Status = "scheduled"; return p }, field: "status"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, service := newService(t)
			_, err := service.Update(context.Background(), tt.post())
			assertValidationError(t, err, tt.field)
		})
	}
}

func TestService_UpdateReturnsNotFound(t *testing.T) {
	repo, service := newService(t)
	ctx := context.Background()
	repo.EXPECT().GetByID(ctx, "missing").Return(nil, posts.ErrNotFound)

	_, err := service.Update(ctx, &posts.Post{
		ID: "missing", Title: "Title", Slug: "title", Status: posts.PostStatusDraft,
	})
	require.ErrorIs(t, err, posts.ErrNotFound)
}

func TestService_UpdateReturnsLookupError(t *testing.T) {
	repo, service := newService(t)
	ctx := context.Background()
	repoErr := errors.New("lookup failed")
	repo.EXPECT().GetByID(ctx, "post-1").Return(nil, repoErr)

	_, err := service.Update(ctx, &posts.Post{
		ID: "post-1", Title: "Title", Slug: "title", Status: posts.PostStatusDraft,
	})
	require.ErrorIs(t, err, repoErr)
}

func TestService_UpdateReturnsRepositoryError(t *testing.T) {
	repo, service := newService(t)
	ctx := context.Background()
	current := testPost("post-1", posts.PostStatusDraft)
	repoErr := errors.New("update failed")
	gomock.InOrder(
		repo.EXPECT().GetByID(ctx, "post-1").Return(&current, nil),
		repo.EXPECT().Update(ctx, gomock.Any()).Return(nil, repoErr),
	)

	_, err := service.Update(ctx, &posts.Post{
		ID: "post-1", Title: "Title", Slug: "title", Status: posts.PostStatusDraft,
	})
	require.ErrorIs(t, err, repoErr)
}

func TestService_Delete(t *testing.T) {
	repo, service := newService(t)
	ctx := context.Background()
	repo.EXPECT().Delete(ctx, "post-1").Return(nil)

	require.NoError(t, service.Delete(ctx, " post-1 "))
}

func TestService_DeleteRejectsBlankID(t *testing.T) {
	_, service := newService(t)
	assertValidationError(t, service.Delete(context.Background(), " \t "), "id")
}

func TestService_DeleteReturnsNotFound(t *testing.T) {
	repo, service := newService(t)
	repo.EXPECT().Delete(gomock.Any(), "missing").Return(posts.ErrNotFound)

	require.ErrorIs(t, service.Delete(context.Background(), "missing"), posts.ErrNotFound)
}

func TestService_DeleteReturnsRepositoryError(t *testing.T) {
	repo, service := newService(t)
	repoErr := errors.New("delete failed")
	repo.EXPECT().Delete(gomock.Any(), "post-1").Return(repoErr)

	require.ErrorIs(t, service.Delete(context.Background(), "post-1"), repoErr)
}

func assertValidationError(t *testing.T, err error, field string) {
	t.Helper()
	require.Error(t, err, "want invalid %s error", field)
	require.ErrorIs(t, err, posts.ErrInvalidPost)
	var validationErr *posts.ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Equal(t, field, validationErr.Field)
}

func testPost(id string, status posts.PostStatus) posts.Post {
	now := time.Now().UTC().Add(-time.Hour)
	post := posts.Post{
		ID: id, Title: "Title " + id, Slug: "title-" + strings.ToLower(id),
		Content: "content", Status: status, CreatedAt: now, UpdatedAt: now,
	}
	if status == posts.PostStatusPublished || status == posts.PostStatusArchived {
		publishedAt := now.Add(time.Minute)
		post.PublishedAt = &publishedAt
	}
	if status == posts.PostStatusArchived {
		archivedAt := now.Add(2 * time.Minute)
		post.ArchivedAt = &archivedAt
	}
	return post
}

func clonePost(post posts.Post) posts.Post {
	post.PublishedAt = cloneTime(post.PublishedAt)
	post.ArchivedAt = cloneTime(post.ArchivedAt)
	return post
}

func clonePostPtr(post *posts.Post) *posts.Post {
	if post == nil {
		return nil
	}
	clone := clonePost(*post)
	return &clone
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}
