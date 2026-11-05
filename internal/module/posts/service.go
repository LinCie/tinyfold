package posts

import (
	"context"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"tinyfold/internal/pkg/validation"
)

//go:generate mockgen -source=service.go -destination=mock/service.go -package=mock
type Service interface {
	Create(ctx context.Context, post *Post) (*Post, error)
	GetMany(ctx context.Context) ([]Post, error)
	GetByID(ctx context.Context, id string) (*Post, error)
	Update(ctx context.Context, post *Post) (*Post, error)
	Delete(ctx context.Context, id string) error
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Create implements [Service].
func (s service) Create(ctx context.Context, post *Post) (*Post, error) {
	if post == nil {
		return nil, invalid("post", "is required")
	}

	title, err := validateTitle(post.Title)
	if err != nil {
		return nil, err
	}
	slug, err := validateSlug(post.Slug)
	if err != nil {
		return nil, err
	}
	if len(post.Content) > MaxContentLength {
		return nil, invalid("content", "is too long")
	}

	status := post.Status
	if status == "" {
		status = PostStatusDraft
	}
	if status != PostStatusDraft {
		return nil, invalid("status", "must be draft when creating a post")
	}

	created := &Post{
		Title: title, Slug: slug, Content: post.Content, Status: status,
	}
	return s.repo.Create(ctx, created)
}

// GetByID implements [Service].
func (s service) GetByID(ctx context.Context, id string) (*Post, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, invalid("id", "is required")
	}

	post, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if post == nil {
		return nil, ErrNotFound
	}
	return post, nil
}

// GetMany implements [Service].
func (s service) GetMany(ctx context.Context) ([]Post, error) {
	posts, err := s.repo.GetMany(ctx)
	if err != nil {
		return nil, err
	}
	if posts == nil {
		posts = []Post{}
	}
	return posts, nil
}

// Update implements [Service].
func (s service) Update(ctx context.Context, post *Post) (*Post, error) {
	if post == nil {
		return nil, invalid("post", "is required")
	}

	id := strings.TrimSpace(post.ID)
	if id == "" {
		return nil, invalid("id", "is required")
	}
	title, err := validateTitle(post.Title)
	if err != nil {
		return nil, err
	}
	slug, err := validateSlug(post.Slug)
	if err != nil {
		return nil, err
	}
	if len(post.Content) > MaxContentLength {
		return nil, invalid("content", "is too long")
	}
	if !validStatus(post.Status) {
		return nil, invalid("status", "is invalid")
	}

	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, ErrNotFound
	}
	if !validTransition(current.Status, post.Status) {
		return nil, invalid("status", "transition is not allowed")
	}

	now := time.Now().UTC()
	updated := &Post{
		ID: id, Title: title, Slug: slug, Content: post.Content, Status: post.Status,
		CreatedAt: current.CreatedAt, UpdatedAt: now,
		PublishedAt: current.PublishedAt, ArchivedAt: current.ArchivedAt,
	}
	if post.Status == PostStatusPublished && updated.PublishedAt == nil {
		publishedAt := now
		updated.PublishedAt = &publishedAt
	}
	if post.Status == PostStatusArchived && updated.ArchivedAt == nil {
		archivedAt := now
		updated.ArchivedAt = &archivedAt
	}

	return s.repo.Update(ctx, updated)
}

// Delete implements [Service].
func (s service) Delete(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return invalid("id", "is required")
	}
	return s.repo.Delete(ctx, id)
}

func validateTitle(title string) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", invalid("title", "is required")
	}
	if utf8.RuneCountInString(title) > MaxTitleLength {
		return "", invalid("title", "is too long")
	}
	return title, nil
}

func validateSlug(slug string) (string, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return "", invalid("slug", "is required")
	}
	if len(slug) > MaxSlugLength || !slugPattern.MatchString(slug) {
		return "", invalid("slug", "must contain lowercase letters, numbers, and single hyphens only")
	}
	return slug, nil
}

func validStatus(status PostStatus) bool {
	return status == PostStatusDraft || status == PostStatusPublished || status == PostStatusArchived
}

func validTransition(from, to PostStatus) bool {
	switch from {
	case PostStatusDraft:
		return validStatus(to)
	case PostStatusPublished:
		return to == PostStatusPublished || to == PostStatusArchived
	case PostStatusArchived:
		return to == PostStatusArchived
	default:
		return false
	}
}

func invalid(field, reason string) error {
	return validation.New(ErrInvalidPost, field, reason)
}
