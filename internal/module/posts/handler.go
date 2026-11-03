package posts

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"tinyfold/internal/httpx"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var post Post
	if err := json.NewDecoder(r.Body).Decode(&post); err != nil {
		httpx.RespondError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON body")
		return
	}

	created, err := h.service.Create(r.Context(), &post)
	if err != nil {
		h.respondError(w, err)
		return
	}
	httpx.RespondJSON(w, http.StatusCreated, created)
}

func (h *Handler) GetMany(w http.ResponseWriter, r *http.Request) {
	posts, err := h.service.GetMany(r.Context())
	if err != nil {
		h.respondError(w, err)
		return
	}
	httpx.RespondJSON(w, http.StatusOK, posts)
}

func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	post, err := h.service.GetByID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.respondError(w, err)
		return
	}
	httpx.RespondJSON(w, http.StatusOK, post)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	var post Post
	if err := json.NewDecoder(r.Body).Decode(&post); err != nil {
		httpx.RespondError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON body")
		return
	}
	post.ID = chi.URLParam(r, "id")

	updated, err := h.service.Update(r.Context(), &post)
	if err != nil {
		h.respondError(w, err)
		return
	}
	httpx.RespondJSON(w, http.StatusOK, updated)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		h.respondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) respondError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidPost):
		httpx.RespondError(w, http.StatusBadRequest, "invalid_post", err.Error())
	case errors.Is(err, ErrNotFound):
		httpx.RespondError(w, http.StatusNotFound, "not_found", err.Error())
	default:
		httpx.RespondError(w, http.StatusInternalServerError, "internal_error", "Internal server error")
	}
}
