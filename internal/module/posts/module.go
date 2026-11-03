package posts

import "github.com/jmoiron/sqlx"

type Module struct {
	Handler *Handler
	Service Service
}

func NewModule(db *sqlx.DB) *Module {
	repo := NewSQLiteRepository(db)
	service := NewService(repo)
	handler := NewHandler(service)

	return &Module{
		Service: service,
		Handler: handler,
	}
}
