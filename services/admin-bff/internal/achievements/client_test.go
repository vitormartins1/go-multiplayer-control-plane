package achievements

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c130n/game-backend/services/admin-bff/internal/domain"
)

func TestForPlayer_MapeiaResposta(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/admin/players/player-42/achievements", r.URL.Path)
		_, _ = w.Write([]byte(`{"achievements":[
			{"id":"first_win","name":"Primeira Vitória","description":"Vença uma","progress":1,"target":1,"unlocked":true,"unlocked_at":"2026-06-29T10:00:00Z"},
			{"id":"ten_wins","name":"Veterano","description":"Vença 10","progress":7,"target":10,"unlocked":false}
		]}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL, srv.Client()).ForPlayer(context.Background(), "player-42")
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "first_win", got[0].ID)
	assert.True(t, got[0].Unlocked)
	assert.False(t, got[0].UnlockedAt.IsZero())
	assert.False(t, got[1].Unlocked)
	assert.Equal(t, 7, got[1].Progress)
	assert.True(t, got[1].UnlockedAt.IsZero())
}

func TestForPlayer_SemProgresso_ListaVazia(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"achievements":[]}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL, srv.Client()).ForPlayer(context.Background(), "novato")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestForPlayer_ServicoFora_ErrAchievementsUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	_, err := New(url, &http.Client{}).ForPlayer(context.Background(), "p1")
	assert.ErrorIs(t, err, domain.ErrAchievementsUnavailable)
}

func TestForPlayer_HTTP500_ErrAchievementsUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := New(srv.URL, srv.Client()).ForPlayer(context.Background(), "p1")
	assert.ErrorIs(t, err, domain.ErrAchievementsUnavailable)
}
