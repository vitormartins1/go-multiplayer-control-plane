package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/c130n/game-backend/services/game-registry/internal/domain"
)

type mockService struct{ mock.Mock }

func (m *mockService) Register(ctx context.Context, slug, name string) (*domain.Game, error) {
	args := m.Called(ctx, slug, name)
	g, _ := args.Get(0).(*domain.Game)
	return g, args.Error(1)
}
func (m *mockService) List(ctx context.Context) ([]domain.Game, error) {
	args := m.Called(ctx)
	games, _ := args.Get(0).([]domain.Game)
	return games, args.Error(1)
}
func (m *mockService) Get(ctx context.Context, id string) (*domain.Game, error) {
	args := m.Called(ctx, id)
	g, _ := args.Get(0).(*domain.Game)
	return g, args.Error(1)
}

func newServer(svc domain.GameService) http.Handler {
	mux := http.NewServeMux()
	NewHandler(svc).Register(mux)
	return mux
}

func do(srv http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	return rr
}

func TestCreate_Valido_201(t *testing.T) {
	svc := new(mockService)
	g := &domain.Game{ID: "id-1", Slug: "tetris", Name: "Tetris"}
	svc.On("Register", mock.Anything, "tetris", "Tetris").Return(g, nil)

	rr := do(newServer(svc), http.MethodPost, "/games", `{"slug":"tetris","name":"Tetris"}`)
	require.Equal(t, http.StatusCreated, rr.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "id-1", resp["id"])
	assert.Equal(t, "tetris", resp["slug"])
}

func TestCreate_SlugInvalido_400(t *testing.T) {
	svc := new(mockService)
	svc.On("Register", mock.Anything, mock.Anything, mock.Anything).Return(nil, domain.ErrInvalidSlug)
	rr := do(newServer(svc), http.MethodPost, "/games", `{"slug":"X","name":"n"}`)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreate_NameInvalido_400(t *testing.T) {
	svc := new(mockService)
	svc.On("Register", mock.Anything, mock.Anything, mock.Anything).Return(nil, domain.ErrInvalidName)
	rr := do(newServer(svc), http.MethodPost, "/games", `{"slug":"tetris","name":""}`)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreate_SlugTaken_409(t *testing.T) {
	svc := new(mockService)
	svc.On("Register", mock.Anything, mock.Anything, mock.Anything).Return(nil, domain.ErrSlugTaken)
	rr := do(newServer(svc), http.MethodPost, "/games", `{"slug":"tetris","name":"Tetris"}`)
	assert.Equal(t, http.StatusConflict, rr.Code)
}

func TestList_200(t *testing.T) {
	svc := new(mockService)
	svc.On("List", mock.Anything).Return([]domain.Game{{ID: "a", Slug: "arkanoid", Name: "Arkanoid"}}, nil)
	rr := do(newServer(svc), http.MethodGet, "/games", "")
	require.Equal(t, http.StatusOK, rr.Code)

	var resp []map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.Len(t, resp, 1)
	assert.Equal(t, "arkanoid", resp[0]["slug"])
}

func TestGet_200(t *testing.T) {
	svc := new(mockService)
	svc.On("Get", mock.Anything, "id-1").Return(&domain.Game{ID: "id-1", Slug: "tetris", Name: "Tetris"}, nil)
	rr := do(newServer(svc), http.MethodGet, "/games/id-1", "")
	require.Equal(t, http.StatusOK, rr.Code)
}

func TestGet_Inexistente_404(t *testing.T) {
	svc := new(mockService)
	svc.On("Get", mock.Anything, "nope").Return(nil, domain.ErrGameNotFound)
	rr := do(newServer(svc), http.MethodGet, "/games/nope", "")
	assert.Equal(t, http.StatusNotFound, rr.Code)
}
