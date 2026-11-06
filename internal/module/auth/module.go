package auth

import (
	"github.com/jmoiron/sqlx"
	"tinyfold/internal/config"
	"tinyfold/internal/module/users"
)

type Module struct {
	Handler *Handler
	Service Service
}

func NewModule(db *sqlx.DB, cfg *config.Config, userService users.Service, passwordService PasswordService) (*Module, error) {
	repo := NewSQLiteRepository(db)
	service, err := NewService(repo, userService, passwordService, cfg)
	if err != nil {
		return nil, err
	}
	handler := NewHandler(service)
	return &Module{Handler: handler, Service: service}, nil
}
