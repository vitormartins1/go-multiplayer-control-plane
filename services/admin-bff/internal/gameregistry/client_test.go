package gameregistry

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c130n/game-backend/services/admin-bff/internal/domain"
)

func TestListGames_Mapeia(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/games", r.URL.Path)
		_, _ = w.Write([]byte(`[{"id":"g1","slug":"tetris","name":"Tetris","createdAt":"2026-06-29T10:00:00Z"}]`))
	}))
	defer srv.Close()

	games, err := New(srv.URL, srv.Client()).ListGames(context.Background())
	require.NoError(t, err)
	require.Len(t, games, 1)
	assert.Equal(t, "tetris", games[0].Slug)
}

func TestCreateGame_201(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"g1","slug":"tetris","name":"Tetris"}`))
	}))
	defer srv.Close()

	g, err := New(srv.URL, srv.Client()).CreateGame(context.Background(), "tetris", "Tetris")
	require.NoError(t, err)
	assert.Equal(t, "g1", g.ID)
}

func TestCreateGame_409_UpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"slug_taken"}`))
	}))
	defer srv.Close()

	_, err := New(srv.URL, srv.Client()).CreateGame(context.Background(), "tetris", "Tetris")
	var ue *domain.UpstreamError
	require.ErrorAs(t, err, &ue)
	assert.Equal(t, http.StatusConflict, ue.Status)
	assert.Equal(t, "slug_taken", ue.Code)
}

func TestCreateGame_Fora_Unavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	_, err := New(url, &http.Client{}).CreateGame(context.Background(), "tetris", "Tetris")
	assert.True(t, errors.Is(err, domain.ErrGameRegistryUnavailable))
}

func TestListGames_HTTP500_Unavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := New(srv.URL, srv.Client()).ListGames(context.Background())
	assert.ErrorIs(t, err, domain.ErrGameRegistryUnavailable)
}

func TestPlayersOf_200(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`[{"playerId":"p1","handle":"Alice","firstSeen":"2026-06-30T12:00:00Z"}]`))
	}))
	defer srv.Close()

	got, err := New(srv.URL, srv.Client()).PlayersOf(context.Background(), "game-1")
	require.NoError(t, err)
	assert.Equal(t, "/games/game-1/players", gotPath)
	require.Len(t, got, 1)
	assert.Equal(t, "p1", got[0].PlayerID)
	assert.Equal(t, "Alice", got[0].Handle)
}

func TestPlayersOf_HTTP500_Unavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := New(srv.URL, srv.Client()).PlayersOf(context.Background(), "game-1")
	assert.ErrorIs(t, err, domain.ErrGameRegistryUnavailable)
}

func TestGamesOfPlayer_200(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`[{"id":"g1","slug":"tetris","name":"Tetris","generatedImageSeed":"tetris"}]`))
	}))
	defer srv.Close()

	got, err := New(srv.URL, srv.Client()).GamesOfPlayer(context.Background(), "p1")
	require.NoError(t, err)
	assert.Equal(t, "/players/p1/games", gotPath)
	require.Len(t, got, 1)
	assert.Equal(t, "tetris", got[0].Slug)
}
