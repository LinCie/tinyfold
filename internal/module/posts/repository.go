package posts

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

//go:generate mockgen -source=repository.go -destination=mock/repository.go -package=mock
type Repository interface {
	Create(ctx context.Context, post *Post) (*Post, error)
	GetMany(ctx context.Context) ([]Post, error)
	GetByID(ctx context.Context, id string) (*Post, error)
	Update(ctx context.Context, post *Post) (*Post, error)
	Delete(ctx context.Context, id string) error
}

type sqliteRepository struct {
	db *sqlx.DB
}

func NewSQLiteRepository(db *sqlx.DB) Repository {
	return &sqliteRepository{db: db}
}

const postColumns = `id, title, slug, content, status, created_at, updated_at, published_at, archived_at`

func (p *sqliteRepository) Create(ctx context.Context, post *Post) (*Post, error) {
	if post == nil {
		return nil, ErrInvalidPost
	}
	created := *post
	created.ID = uuid.NewString()
	created.CreatedAt = time.Now().UTC()
	created.UpdatedAt = created.CreatedAt

	_, err := p.db.ExecContext(ctx,
		`INSERT INTO posts (`+postColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		created.ID, created.Title, created.Slug, created.Content, created.Status,
		created.CreatedAt, created.UpdatedAt, created.PublishedAt, created.ArchivedAt,
	)
	if err != nil {
		return nil, err
	}
	return &created, nil
}

func (p *sqliteRepository) GetByID(ctx context.Context, id string) (*Post, error) {
	var post Post
	err := p.db.GetContext(ctx, &post, `SELECT `+postColumns+` FROM posts WHERE id = ?`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &post, nil
}

func (p *sqliteRepository) GetMany(ctx context.Context) ([]Post, error) {
	var posts []Post
	if err := p.db.SelectContext(ctx, &posts, `SELECT `+postColumns+` FROM posts`); err != nil {
		return nil, err
	}
	return posts, nil
}

func (p *sqliteRepository) Update(ctx context.Context, post *Post) (*Post, error) {
	if post == nil {
		return nil, ErrInvalidPost
	}
	result, err := p.db.ExecContext(ctx,
		`UPDATE posts SET title = ?, slug = ?, content = ?, status = ?, created_at = ?, updated_at = ?, published_at = ?, archived_at = ? WHERE id = ?`,
		post.Title, post.Slug, post.Content, post.Status, post.CreatedAt, post.UpdatedAt,
		post.PublishedAt, post.ArchivedAt, post.ID,
	)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, ErrNotFound
	}
	updated := *post
	return &updated, nil
}

func (p *sqliteRepository) Delete(ctx context.Context, id string) error {
	result, err := p.db.ExecContext(ctx, `DELETE FROM posts WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}
