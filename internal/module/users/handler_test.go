package users_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"tinyfold/internal/module/users"
	usersMock "tinyfold/internal/module/users/mock"
)

func handlerForTest(t *testing.T, service users.Service) http.Handler {
	t.Helper()
	handler := users.NewHandler(service)
	router := chi.NewRouter()
	router.Post("/users", handler.Create)
	router.Get("/users", handler.GetMany)
	router.Get("/users/{id}", handler.GetByID)
	router.Put("/users/{id}", handler.Update)
	router.Delete("/users/{id}", handler.Delete)
	return router
}

func serve(handler http.Handler, method, target string, body io.Reader) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(method, target, body))
	return response
}

func TestHandler_Create(t *testing.T) {
	service := usersMock.NewMockService(gomock.NewController(t))
	handler := handlerForTest(t, service)
	input := &users.User{Username: "jane-doe", Email: "jane@example.com", DisplayName: "Jane Doe", Bio: "bio"}
	created := &users.User{ID: "user-1", Username: input.Username, Email: input.Email, DisplayName: input.DisplayName, Bio: input.Bio}
	service.EXPECT().Create(gomock.Any(), input).Return(created, nil)

	response := serve(handler, http.MethodPost, "/users", jsonBody(t, input))

	assert.Equal(t, http.StatusCreated, response.Code)
	assertUserResponse(t, response, created)
}

func TestHandler_CreateRejectsMalformedJSON(t *testing.T) {
	service := usersMock.NewMockService(gomock.NewController(t))
	handler := handlerForTest(t, service)

	response := serve(handler, http.MethodPost, "/users", bytes.NewBufferString(`{"username":`))

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestHandler_CreateMapsServiceErrors(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
		want int
	}{
		{name: "validation error", err: users.ErrInvalidUser, want: http.StatusBadRequest},
		{name: "unexpected error", err: errors.New("database unavailable"), want: http.StatusInternalServerError},
	} {
		t.Run(tt.name, func(t *testing.T) {
			service := usersMock.NewMockService(gomock.NewController(t))
			handler := handlerForTest(t, service)
			input := &users.User{Username: "jane-doe", Email: "jane@example.com", DisplayName: "Jane Doe"}
			service.EXPECT().Create(gomock.Any(), input).Return(nil, tt.err)

			response := serve(handler, http.MethodPost, "/users", jsonBody(t, input))

			assert.Equal(t, tt.want, response.Code)
		})
	}
}

func TestHandler_GetMany(t *testing.T) {
	service := usersMock.NewMockService(gomock.NewController(t))
	handler := handlerForTest(t, service)
	want := []users.User{{ID: "user-1", Username: "jane-doe", Email: "jane@example.com", DisplayName: "Jane Doe"}}
	service.EXPECT().GetMany(gomock.Any()).Return(want, nil)

	response := serve(handler, http.MethodGet, "/users", nil)

	assert.Equal(t, http.StatusOK, response.Code)
	var body struct {
		Data []users.UserResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	wantResponse := make([]users.UserResponse, 0, len(want))
	for _, u := range want {
		wantResponse = append(wantResponse, users.UserResponse(u))
	}
	assert.Equal(t, wantResponse, body.Data)
}

func TestHandler_GetManyMapsServiceError(t *testing.T) {
	service := usersMock.NewMockService(gomock.NewController(t))
	handler := handlerForTest(t, service)
	service.EXPECT().GetMany(gomock.Any()).Return(nil, errors.New("database unavailable"))

	response := serve(handler, http.MethodGet, "/users", nil)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
}

func TestHandler_GetByID(t *testing.T) {
	service := usersMock.NewMockService(gomock.NewController(t))
	handler := handlerForTest(t, service)
	want := &users.User{ID: "user-1", Username: "jane-doe", Email: "jane@example.com", DisplayName: "Jane Doe"}
	service.EXPECT().GetByID(gomock.Any(), "user-1").Return(want, nil)

	response := serve(handler, http.MethodGet, "/users/user-1", nil)

	assert.Equal(t, http.StatusOK, response.Code)
	assertUserResponse(t, response, want)
}

func TestHandler_GetByIDReturnsNotFound(t *testing.T) {
	service := usersMock.NewMockService(gomock.NewController(t))
	handler := handlerForTest(t, service)
	service.EXPECT().GetByID(gomock.Any(), "missing").Return(nil, users.ErrNotFound)

	response := serve(handler, http.MethodGet, "/users/missing", nil)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestHandler_Update(t *testing.T) {
	service := usersMock.NewMockService(gomock.NewController(t))
	handler := handlerForTest(t, service)
	input := &users.User{Username: "jane-doe-2", Email: "jane2@example.com", DisplayName: "Jane Updated", Bio: "bio"}
	updated := &users.User{ID: "user-1", Username: input.Username, Email: input.Email, DisplayName: input.DisplayName, Bio: input.Bio}
	service.EXPECT().Update(gomock.Any(), gomock.Cond(func(got *users.User) bool {
		return got.ID == "user-1" && got.Username == input.Username && got.Email == input.Email && got.DisplayName == input.DisplayName && got.Bio == input.Bio
	})).Return(updated, nil)

	response := serve(handler, http.MethodPut, "/users/user-1", jsonBody(t, input))

	assert.Equal(t, http.StatusOK, response.Code)
	assertUserResponse(t, response, updated)
}

func TestHandler_UpdateRejectsMalformedJSON(t *testing.T) {
	service := usersMock.NewMockService(gomock.NewController(t))
	handler := handlerForTest(t, service)

	response := serve(handler, http.MethodPut, "/users/user-1", bytes.NewBufferString("not-json"))

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestHandler_Delete(t *testing.T) {
	service := usersMock.NewMockService(gomock.NewController(t))
	handler := handlerForTest(t, service)
	service.EXPECT().Delete(gomock.Any(), "user-1").Return(nil)

	response := serve(handler, http.MethodDelete, "/users/user-1", nil)

	assert.Equal(t, http.StatusNoContent, response.Code)
	assert.Empty(t, response.Body.String())
}

func TestHandler_DeleteReturnsNotFound(t *testing.T) {
	service := usersMock.NewMockService(gomock.NewController(t))
	handler := handlerForTest(t, service)
	service.EXPECT().Delete(gomock.Any(), "missing").Return(users.ErrNotFound)

	response := serve(handler, http.MethodDelete, "/users/missing", nil)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func jsonBody(t *testing.T, value any) *bytes.Reader {
	t.Helper()
	body, err := json.Marshal(value)
	require.NoError(t, err)
	return bytes.NewReader(body)
}

func assertUserResponse(t *testing.T, response *httptest.ResponseRecorder, want *users.User) {
	t.Helper()
	var body struct {
		Data *users.UserResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	wantResponse := users.UserResponse(*want)
	assert.Equal(t, &wantResponse, body.Data)
}
