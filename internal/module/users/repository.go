package users

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/segmentio/ksuid"
	"tinyfold/internal/model"
)

//go:generate mockgen -source=repository.go -destination=mock/repository.go -package=mock
type Repository interface {
	Create(ctx context.Context, user *model.User) (*model.User, error)
	GetMany(ctx context.Context) ([]model.User, error)
	GetByID(ctx context.Context, id string) (*model.User, error)
	Update(ctx context.Context, user *model.User) (*model.User, error)
	Delete(ctx context.Context, id string) error
}

type sqliteRepository struct {
	db *sqlx.DB
}

func NewSQLiteRepository(db *sqlx.DB) Repository {
	return &sqliteRepository{db: db}
}

const userColumns = `id, username, email, display_name, bio, role, status, created_at, updated_at`

func (r *sqliteRepository) Create(ctx context.Context, user *model.User) (*model.User, error) {
	if user == nil {
		return nil, ErrInvalidUser
	}
	created := *user
	created.ID = ksuid.New().String()
	created.CreatedAt = time.Now().UTC()
	created.UpdatedAt = created.CreatedAt

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO users (`+userColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		created.ID, created.Username, created.Email, created.DisplayName, created.Bio,
		created.Role, created.Status, created.CreatedAt, created.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &created, nil
}

func (r *sqliteRepository) GetByID(ctx context.Context, id string) (*model.User, error) {
	var user model.User
	err := r.db.GetContext(ctx, &user, `SELECT `+userColumns+` FROM users WHERE id = ?`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *sqliteRepository) GetMany(ctx context.Context) ([]model.User, error) {
	var users []model.User
	if err := r.db.SelectContext(ctx, &users, `SELECT `+userColumns+` FROM users`); err != nil {
		return nil, err
	}
	return users, nil
}

func (r *sqliteRepository) Update(ctx context.Context, user *model.User) (*model.User, error) {
	if user == nil {
		return nil, ErrInvalidUser
	}
	result, err := r.db.ExecContext(ctx,
		`UPDATE users SET username = ?, email = ?, display_name = ?, bio = ?, role = ?, status = ?, created_at = ?, updated_at = ? WHERE id = ?`,
		user.Username, user.Email, user.DisplayName, user.Bio, user.Role, user.Status,
		user.CreatedAt, user.UpdatedAt, user.ID,
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
	updated := *user
	return &updated, nil
}

func (r *sqliteRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
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
