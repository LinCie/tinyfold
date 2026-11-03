package router

import (
	"net/http"
	"tinyfold/internal/module/posts"
	"tinyfold/internal/module/users"

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
	usersModule := users.NewModule(db)

	mountCRUD(r, "/posts", postsModule.Handler)
	mountCRUD(r, "/users", usersModule.Handler)

	return r
}

// crudHandler is implemented by each module's Handler (Create/GetMany/
// GetByID/Update/Delete with identical signatures).
type crudHandler interface {
	Create(http.ResponseWriter, *http.Request)
	GetMany(http.ResponseWriter, *http.Request)
	GetByID(http.ResponseWriter, *http.Request)
	Update(http.ResponseWriter, *http.Request)
	Delete(http.ResponseWriter, *http.Request)
}

func mountCRUD(r chi.Router, pattern string, h crudHandler) {
	r.Route(pattern, func(r chi.Router) {
		r.Post("/", h.Create)
		r.Get("/", h.GetMany)
		r.Get("/{id}", h.GetByID)
		r.Put("/{id}", h.Update)
		r.Delete("/{id}", h.Delete)
	})
}
