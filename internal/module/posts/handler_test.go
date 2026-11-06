package posts_test

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

	"tinyfold/internal/model"
	"tinyfold/internal/module/posts"
	postsMock "tinyfold/internal/module/posts/mock"
)

func handlerForTest(t *testing.T, service posts.Service) http.Handler {
	t.Helper()
	handler := posts.NewHandler(service)
	router := chi.NewRouter()
	router.Post("/posts", handler.Create)
	router.Get("/posts", handler.GetMany)
	router.Get("/posts/{id}", handler.GetByID)
	router.Put("/posts/{id}", handler.Update)
	router.Delete("/posts/{id}", handler.Delete)
	return router
}

func serve(handler http.Handler, method, target string, body io.Reader) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(method, target, body))
	return response
}

func TestHandler_Create(t *testing.T) {
	service := postsMock.NewMockService(gomock.NewController(t))
	handler := handlerForTest(t, service)
	input := &model.Post{Title: "A title", Slug: "a-title", Content: "Body"}
	created := &model.Post{ID: "post-1", Title: input.Title, Slug: input.Slug, Content: input.Content}
	service.EXPECT().Create(gomock.Any(), input).Return(created, nil)

	response := serve(handler, http.MethodPost, "/posts", jsonBody(t, input))

	assert.Equal(t, http.StatusCreated, response.Code)
	assertPostResponse(t, response, created)
}

func TestHandler_CreateRejectsMalformedJSON(t *testing.T) {
	service := postsMock.NewMockService(gomock.NewController(t))
	handler := handlerForTest(t, service)

	response := serve(handler, http.MethodPost, "/posts", bytes.NewBufferString(`{"title":`))

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestHandler_CreateMapsServiceErrors(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
		want int
	}{
		{name: "validation error", err: posts.ErrInvalidPost, want: http.StatusBadRequest},
		{name: "unexpected error", err: errors.New("database unavailable"), want: http.StatusInternalServerError},
	} {
		t.Run(tt.name, func(t *testing.T) {
			service := postsMock.NewMockService(gomock.NewController(t))
			handler := handlerForTest(t, service)
			input := &model.Post{Title: "A title", Slug: "a-title"}
			service.EXPECT().Create(gomock.Any(), input).Return(nil, tt.err)

			response := serve(handler, http.MethodPost, "/posts", jsonBody(t, input))

			assert.Equal(t, tt.want, response.Code)
		})
	}
}

func TestHandler_GetMany(t *testing.T) {
	service := postsMock.NewMockService(gomock.NewController(t))
	handler := handlerForTest(t, service)
	want := []model.Post{{ID: "post-1", Title: "A title", Slug: "a-title"}}
	service.EXPECT().GetMany(gomock.Any()).Return(want, nil)

	response := serve(handler, http.MethodGet, "/posts", nil)

	assert.Equal(t, http.StatusOK, response.Code)
	var body struct {
		Data []model.Post `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	assert.Equal(t, want, body.Data)
}

func TestHandler_GetManyMapsServiceError(t *testing.T) {
	service := postsMock.NewMockService(gomock.NewController(t))
	handler := handlerForTest(t, service)
	service.EXPECT().GetMany(gomock.Any()).Return(nil, errors.New("database unavailable"))

	response := serve(handler, http.MethodGet, "/posts", nil)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
}

func TestHandler_GetByID(t *testing.T) {
	service := postsMock.NewMockService(gomock.NewController(t))
	handler := handlerForTest(t, service)
	want := &model.Post{ID: "post-1", Title: "A title", Slug: "a-title"}
	service.EXPECT().GetByID(gomock.Any(), "post-1").Return(want, nil)

	response := serve(handler, http.MethodGet, "/posts/post-1", nil)

	assert.Equal(t, http.StatusOK, response.Code)
	assertPostResponse(t, response, want)
}

func TestHandler_GetByIDReturnsNotFound(t *testing.T) {
	service := postsMock.NewMockService(gomock.NewController(t))
	handler := handlerForTest(t, service)
	service.EXPECT().GetByID(gomock.Any(), "missing").Return(nil, posts.ErrNotFound)

	response := serve(handler, http.MethodGet, "/posts/missing", nil)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestHandler_Update(t *testing.T) {
	service := postsMock.NewMockService(gomock.NewController(t))
	handler := handlerForTest(t, service)
	input := &model.Post{Title: "Updated title", Slug: "updated-title", Content: "Body"}
	updated := &model.Post{ID: "post-1", Title: input.Title, Slug: input.Slug, Content: input.Content}
	service.EXPECT().Update(gomock.Any(), gomock.Cond(func(got *model.Post) bool {
		return got.ID == "post-1" && got.Title == input.Title && got.Slug == input.Slug && got.Content == input.Content
	})).Return(updated, nil)

	response := serve(handler, http.MethodPut, "/posts/post-1", jsonBody(t, input))

	assert.Equal(t, http.StatusOK, response.Code)
	assertPostResponse(t, response, updated)
}

func TestHandler_UpdateRejectsMalformedJSON(t *testing.T) {
	service := postsMock.NewMockService(gomock.NewController(t))
	handler := handlerForTest(t, service)

	response := serve(handler, http.MethodPut, "/posts/post-1", bytes.NewBufferString("not-json"))

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestHandler_Delete(t *testing.T) {
	service := postsMock.NewMockService(gomock.NewController(t))
	handler := handlerForTest(t, service)
	service.EXPECT().Delete(gomock.Any(), "post-1").Return(nil)

	response := serve(handler, http.MethodDelete, "/posts/post-1", nil)

	assert.Equal(t, http.StatusNoContent, response.Code)
	assert.Empty(t, response.Body.String())
}

func TestHandler_DeleteReturnsNotFound(t *testing.T) {
	service := postsMock.NewMockService(gomock.NewController(t))
	handler := handlerForTest(t, service)
	service.EXPECT().Delete(gomock.Any(), "missing").Return(posts.ErrNotFound)

	response := serve(handler, http.MethodDelete, "/posts/missing", nil)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func jsonBody(t *testing.T, value any) *bytes.Reader {
	t.Helper()
	body, err := json.Marshal(value)
	require.NoError(t, err)
	return bytes.NewReader(body)
}

func assertPostResponse(t *testing.T, response *httptest.ResponseRecorder, want *model.Post) {
	t.Helper()
	var body struct {
		Data *model.Post `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	assert.Equal(t, want, body.Data)
}
