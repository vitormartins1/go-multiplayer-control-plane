package gameregistry

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-game-backend/services/admin-bff/internal/domain"
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

func TestGetMultiplayerProfile_200_RawPassthrough(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"archetype":"queue_arena","topology":"dedicated","params":{"structure":"ffa"}}`))
	}))
	defer srv.Close()

	raw, err := New(srv.URL, srv.Client()).GetMultiplayerProfile(context.Background(), "g1")
	require.NoError(t, err)
	assert.Equal(t, "/games/g1/multiplayer-profile", gotPath)
	assert.JSONEq(t, `{"archetype":"queue_arena","topology":"dedicated","params":{"structure":"ffa"}}`, string(raw))
}

func TestGetMultiplayerProfile_404_UpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"profile_not_found"}`))
	}))
	defer srv.Close()

	_, err := New(srv.URL, srv.Client()).GetMultiplayerProfile(context.Background(), "g1")
	var ue *domain.UpstreamError
	require.ErrorAs(t, err, &ue)
	assert.Equal(t, http.StatusNotFound, ue.Status)
	assert.Equal(t, "profile_not_found", ue.Code)
}

func TestSetMultiplayerProfile_PutRaw(t *testing.T) {
	var gotMethod, gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotMethod, gotPath, gotBody = r.Method, r.URL.Path, string(b)
		_, _ = w.Write(b) // eco: game-registry devolve o perfil persistido
	}))
	defer srv.Close()

	body := `{"archetype":"queue_arena","topology":"dedicated","params":{"structure":"teams","teamCount":2,"teamSize":3}}`
	raw, err := New(srv.URL, nil).SetMultiplayerProfile(context.Background(), "g1", []byte(body))
	require.NoError(t, err)
	assert.Equal(t, http.MethodPut, gotMethod)
	assert.Equal(t, "/games/g1/multiplayer-profile", gotPath)
	assert.JSONEq(t, body, gotBody)
	assert.JSONEq(t, body, string(raw))
}

func TestSetMultiplayerProfile_400_UpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_profile_params"}`))
	}))
	defer srv.Close()

	_, err := New(srv.URL, srv.Client()).SetMultiplayerProfile(context.Background(), "g1", []byte(`{}`))
	var ue *domain.UpstreamError
	require.ErrorAs(t, err, &ue)
	assert.Equal(t, http.StatusBadRequest, ue.Status)
	assert.Equal(t, "invalid_profile_params", ue.Code)
}

func TestRenameGame_PutName(t *testing.T) {
	var gotMethod, gotPath, gotBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotMethod, gotPath, gotBody = r.Method, r.URL.Path, string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"g1","slug":"orb","name":"Novo"}`))
	}))
	defer upstream.Close()

	g, err := New(upstream.URL, nil).RenameGame(context.Background(), "g1", "Novo")
	require.NoError(t, err)
	assert.Equal(t, http.MethodPut, gotMethod)
	assert.Equal(t, "/games/g1", gotPath)
	assert.Contains(t, gotBody, `"name":"Novo"`)
	assert.Equal(t, "Novo", g.Name)
}
