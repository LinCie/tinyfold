package users

import (
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
	var user User
	if !httpx.DecodeJSON(w, r, &user) {
		return
	}

	created, err := h.service.Create(r.Context(), &user)
	if err != nil {
		h.respondError(w, err)
		return
	}
	httpx.RespondJSON(w, http.StatusCreated, UserResponse(*created))
}

func (h *Handler) GetMany(w http.ResponseWriter, r *http.Request) {
	users, err := h.service.GetMany(r.Context())
	if err != nil {
		h.respondError(w, err)
		return
	}
	responses := make([]UserResponse, 0, len(users))
	for _, u := range users {
		responses = append(responses, UserResponse(u))
	}
	httpx.RespondJSON(w, http.StatusOK, responses)
}

func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	user, err := h.service.GetByID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.respondError(w, err)
		return
	}
	httpx.RespondJSON(w, http.StatusOK, UserResponse(*user))
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	var user User
	if !httpx.DecodeJSON(w, r, &user) {
		return
	}
	user.ID = chi.URLParam(r, "id")

	updated, err := h.service.Update(r.Context(), &user)
	if err != nil {
		h.respondError(w, err)
		return
	}
	httpx.RespondJSON(w, http.StatusOK, UserResponse(*updated))
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
	case errors.Is(err, ErrInvalidUser):
		httpx.RespondError(w, http.StatusBadRequest, "invalid_user", err.Error())
	case errors.Is(err, ErrNotFound):
		httpx.RespondError(w, http.StatusNotFound, "not_found", err.Error())
	default:
		httpx.RespondError(w, http.StatusInternalServerError, "internal_error", "Internal server error")
	}
}
