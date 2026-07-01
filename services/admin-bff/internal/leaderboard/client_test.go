package leaderboard

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-game-backend/services/admin-bff/internal/domain"
)

func TestTopForGame_200(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		_, _ = w.Write([]byte(`{"entries":[{"rank":1,"playerId":"bob","score":300},{"rank":2,"playerId":"alice","score":120}]}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL, srv.Client()).TopForGame(context.Background(), "game-1", 10)
	require.NoError(t, err)
	assert.Equal(t, "/admin/games/game-1/leaderboard", gotPath)
	assert.Equal(t, "limit=10", gotQuery)
	require.Len(t, got, 2)
	assert.Equal(t, domain.LeaderboardEntry{Rank: 1, PlayerID: "bob", Score: 300}, got[0])
	assert.Equal(t, "alice", got[1].PlayerID)
}

func TestTopForGame_Fora_ErrLeaderboardUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := New(srv.URL, srv.Client()).TopForGame(context.Background(), "game-1", 0)
	assert.True(t, errors.Is(err, domain.ErrLeaderboardUnavailable))
}
