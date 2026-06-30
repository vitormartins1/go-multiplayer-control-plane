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

type mockPresence struct{ mock.Mock }

func (m *mockPresence) Mark(ctx context.Context, gameID, playerID, handle string) error {
	return m.Called(ctx, gameID, playerID, handle).Error(0)
}
func (m *mockPresence) PlayersOf(ctx context.Context, gameID string) ([]domain.Presence, error) {
	args := m.Called(ctx, gameID)
	ps, _ := args.Get(0).([]domain.Presence)
	return ps, args.Error(1)
}

func newServer(svc domain.GameService) http.Handler {
	return newServerWith(svc, new(mockPresence))
}

func newServerWith(svc domain.GameService, presence domain.PresenceService) http.Handler {
	mux := http.NewServeMux()
	NewHandler(svc, presence).Register(mux)
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

func TestMarkPresence_204(t *testing.T) {
	pres := new(mockPresence)
	pres.On("Mark", mock.Anything, "g1", "p1", "Alice").Return(nil)
	rr := do(newServerWith(new(mockService), pres), http.MethodPut, "/games/g1/players/p1", `{"handle":"Alice"}`)
	assert.Equal(t, http.StatusNoContent, rr.Code)
	pres.AssertExpectations(t)
}

func TestMarkPresence_JogoInexistente_404(t *testing.T) {
	pres := new(mockPresence)
	pres.On("Mark", mock.Anything, "nope", "p1", "Alice").Return(domain.ErrGameNotFound)
	rr := do(newServerWith(new(mockService), pres), http.MethodPut, "/games/nope/players/p1", `{"handle":"Alice"}`)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestListPlayers_200(t *testing.T) {
	pres := new(mockPresence)
	pres.On("PlayersOf", mock.Anything, "g1").Return([]domain.Presence{{PlayerID: "p1", Handle: "Alice"}}, nil)
	rr := do(newServerWith(new(mockService), pres), http.MethodGet, "/games/g1/players", "")
	require.Equal(t, http.StatusOK, rr.Code)

	var resp []map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.Len(t, resp, 1)
	assert.Equal(t, "p1", resp[0]["playerId"])
	assert.Equal(t, "Alice", resp[0]["handle"])
}
