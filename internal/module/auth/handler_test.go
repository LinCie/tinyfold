package auth_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tinyfold/internal/model"
	"tinyfold/internal/module/auth"
	"tinyfold/internal/pkg/jwt"
)

type handlerService struct {
	register func(context.Context, *model.User, string) (*model.User, error)
	login    func(context.Context, string, string) (jwt.Pair, error)
	refresh  func(context.Context, string) (jwt.Pair, error)
	logout   func(context.Context, string) error
}

func (s *handlerService) Register(ctx context.Context, user *model.User, password string) (*model.User, error) {
	return s.register(ctx, user, password)
}
func (s *handlerService) Login(ctx context.Context, userID, password string) (jwt.Pair, error) {
	return s.login(ctx, userID, password)
}
func (s *handlerService) Issue(context.Context, string) (jwt.Pair, error) {
	panic("unexpected Issue call")
}
func (s *handlerService) Refresh(ctx context.Context, token string) (jwt.Pair, error) {
	return s.refresh(ctx, token)
}
func (s *handlerService) Logout(ctx context.Context, token string) error {
	return s.logout(ctx, token)
}

func serveAuth(h http.HandlerFunc, method, body string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	req := httptest.NewRequest(method, "/auth", strings.NewReader(body))
	h.ServeHTTP(response, req)
	return response
}

func TestHandler_Register(t *testing.T) {
	service := &handlerService{register: func(_ context.Context, user *model.User, password string) (*model.User, error) {
		assert.Equal(t, "secret", password)
		assert.Equal(t, "jane-doe", user.Username)
		return &model.User{ID: "u1", Username: user.Username, Status: model.UserStatusInvited}, nil
	}}
	response := serveAuth(auth.NewHandler(service).Register, http.MethodPost,
		`{"user":{"Username":"jane-doe"},"password":"secret"}`)
	assert.Equal(t, http.StatusCreated, response.Code)
	assert.NotContains(t, response.Body.String(), "PasswordHash")
}

func TestHandler_Login(t *testing.T) {
	service := &handlerService{login: func(_ context.Context, userID, password string) (jwt.Pair, error) {
		assert.Equal(t, "u1", userID)
		assert.Equal(t, "secret", password)
		return jwt.Pair{AccessToken: "access", RefreshToken: "refresh"}, nil
	}}
	response := serveAuth(auth.NewHandler(service).Login, http.MethodPost, `{"user_id":"u1","password":"secret"}`)
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "access")
}

func TestHandler_RefreshAndLogout(t *testing.T) {
	service := &handlerService{
		refresh: func(_ context.Context, token string) (jwt.Pair, error) {
			assert.Equal(t, "old-refresh", token)
			return jwt.Pair{AccessToken: "new-access", RefreshToken: "new-refresh"}, nil
		},
		logout: func(_ context.Context, token string) error {
			assert.Equal(t, "old-refresh", token)
			return nil
		},
	}
	h := auth.NewHandler(service)
	response := serveAuth(h.Refresh, http.MethodPost, `{"refresh_token":"old-refresh"}`)
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "new-refresh")

	response = serveAuth(h.Logout, http.MethodPost, `{"refresh_token":"old-refresh"}`)
	assert.Equal(t, http.StatusNoContent, response.Code)
}

func TestHandler_RejectsMalformedOrMissingRefreshToken(t *testing.T) {
	h := auth.NewHandler(&handlerService{})
	for _, body := range []string{`{"refresh_token":`, `{}`} {
		response := serveAuth(h.Refresh, http.MethodPost, body)
		assert.Equal(t, http.StatusBadRequest, response.Code)
	}
	response := serveAuth(h.Logout, http.MethodPost, `{}`)
	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestHandler_MapsAuthErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		method func(*auth.Handler) http.HandlerFunc
		err    error
		status int
	}{
		{name: "invalid refresh", method: func(h *auth.Handler) http.HandlerFunc { return h.Refresh }, err: auth.ErrInvalidRefreshToken, status: http.StatusUnauthorized},
		{name: "invalid credentials", method: func(h *auth.Handler) http.HandlerFunc { return h.Login }, err: auth.ErrInvalidCredentials, status: http.StatusUnauthorized},
		{name: "inactive user", method: func(h *auth.Handler) http.HandlerFunc { return h.Login }, err: auth.ErrInactiveUser, status: http.StatusForbidden},
		{name: "unexpected error", method: func(h *auth.Handler) http.HandlerFunc { return h.Logout }, err: errors.New("database failure"), status: http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := &handlerService{
				refresh: func(context.Context, string) (jwt.Pair, error) { return jwt.Pair{}, tc.err },
				login:   func(context.Context, string, string) (jwt.Pair, error) { return jwt.Pair{}, tc.err },
				logout:  func(context.Context, string) error { return tc.err },
			}
			response := serveAuth(tc.method(auth.NewHandler(service)), http.MethodPost, `{"refresh_token":"token","user_id":"u1","password":"x"}`)
			assert.Equal(t, tc.status, response.Code)
		})
	}
}

func TestHandler_RegisterMalformedJSON(t *testing.T) {
	h := auth.NewHandler(&handlerService{})
	response := serveAuth(h.Register, http.MethodPost, `{"user":`)
	assert.Equal(t, http.StatusBadRequest, response.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
}
