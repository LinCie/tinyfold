package auth

import (
	"errors"
	"net/http"

	"tinyfold/internal/model"
	"tinyfold/internal/module/users"
	"tinyfold/internal/pkg/httpx"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type registerRequest struct {
	User     model.User `json:"user"`
	Password string     `json:"password"`
}

type loginRequest struct {
	UserID   string `json:"user_id"`
	Password string `json:"password"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var request registerRequest
	if !httpx.DecodeJSON(w, r, &request) {
		return
	}
	user, err := h.service.Register(r.Context(), &request.User, request.Password)
	if err != nil {
		h.respondError(w, err)
		return
	}
	httpx.RespondJSON(w, http.StatusCreated, users.ToUserResponse(*user))
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var request loginRequest
	if !httpx.DecodeJSON(w, r, &request) {
		return
	}
	pair, err := h.service.Login(r.Context(), request.UserID, request.Password)
	if err != nil {
		h.respondError(w, err)
		return
	}
	httpx.RespondJSON(w, http.StatusOK, pair)
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var request refreshRequest
	if !httpx.DecodeJSON(w, r, &request) {
		return
	}
	if request.RefreshToken == "" {
		httpx.RespondError(w, http.StatusBadRequest, "invalid_request", "refresh_token is required")
		return
	}

	pair, err := h.service.Refresh(r.Context(), request.RefreshToken)
	if err != nil {
		h.respondError(w, err)
		return
	}
	httpx.RespondJSON(w, http.StatusOK, pair)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var request refreshRequest
	if !httpx.DecodeJSON(w, r, &request) {
		return
	}
	if request.RefreshToken == "" {
		httpx.RespondError(w, http.StatusBadRequest, "invalid_request", "refresh_token is required")
		return
	}

	if err := h.service.Logout(r.Context(), request.RefreshToken); err != nil {
		h.respondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) respondError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidRefreshToken):
		httpx.RespondError(w, http.StatusUnauthorized, "invalid_refresh_token", "Refresh token is invalid or expired")
		return
	case errors.Is(err, ErrInvalidCredentials):
		httpx.RespondError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid user ID or password")
		return
	case errors.Is(err, ErrInactiveUser):
		httpx.RespondError(w, http.StatusForbidden, "inactive_user", "User is not active")
		return
	}
	httpx.RespondError(w, http.StatusInternalServerError, "internal_error", "Internal server error")
}
