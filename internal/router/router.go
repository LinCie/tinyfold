package router

import (
	"net/http"
	"tinyfold/internal/module/posts"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jmoiron/sqlx"
)

func NewRouter(db *sqlx.DB) http.Handler {
	r := chi.NewRouter()

	r.Use(
		chimiddleware.Recoverer,
		chimiddleware.StripSlashes,
		chimiddleware.RequestID,
		chimiddleware.Logger,
	)

	postsModule := posts.NewModule(db)

	r.Route("/posts", func(r chi.Router) {
		r.Post("/", postsModule.Handler.Create)
		r.Get("/", postsModule.Handler.GetMany)
		r.Get("/{id}", postsModule.Handler.GetByID)
		r.Put("/{id}", postsModule.Handler.Update)
		r.Delete("/{id}", postsModule.Handler.Delete)
	})

	return r
}
