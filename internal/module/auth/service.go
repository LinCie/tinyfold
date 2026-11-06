package auth

import (
	"context"

	"tinyfold/internal/model"
)

type UserService interface {
	Create(ctx context.Context, user *model.User) (*model.User, error)
	GetByID(ctx context.Context, id string) (*model.User, error)
}

type PasswordService interface {
	Hash(ctx context.Context, password string) (string, error)
	Verify(ctx context.Context, password string, hash string) (bool, error)
}

type Service interface {
}
