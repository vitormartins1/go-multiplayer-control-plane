package achievements

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-game-backend/services/admin-bff/internal/domain"
)

func TestCatalogForGame_Mapeia(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/admin/games/g1/achievements", r.URL.Path)
		_, _ = w.Write([]byte(`{"achievements":[{"id":"first_win","name":"Primeira","description":"x","target":1}]}`))
	}))
	defer srv.Close()

	cat, err := New(srv.URL, srv.Client()).CatalogForGame(context.Background(), "g1")
	require.NoError(t, err)
	require.Len(t, cat, 1)
	assert.Equal(t, "first_win", cat[0].ID)
}

func TestPlayerAchievements_Mapeia(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/admin/games/g1/players/p1/achievements", r.URL.Path)
		_, _ = w.Write([]byte(`{"achievements":[{"id":"first_win","name":"P","description":"x","progress":1,"target":1,"unlocked":true,"unlocked_at":"2026-06-29T10:00:00Z"}]}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL, srv.Client()).PlayerAchievements(context.Background(), "g1", "p1")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.True(t, got[0].Unlocked)
	assert.False(t, got[0].UnlockedAt.IsZero())
}

func TestCreateAchievement_201(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/admin/games/g1/achievements", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"speedrun","name":"Speed","description":"x","target":1}`))
	}))
	defer srv.Close()

	err := New(srv.URL, srv.Client()).CreateAchievement(context.Background(), "g1", domain.Achievement{
		ID: "speedrun", Name: "Speed", EventType: "fast_win", Target: 1,
	})
	require.NoError(t, err)
}

func TestCreateAchievement_409_UpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"achievement_exists"}`))
	}))
	defer srv.Close()

	err := New(srv.URL, srv.Client()).CreateAchievement(context.Background(), "g1", domain.Achievement{ID: "x", Name: "x", EventType: "kill", Target: 1})
	var ue *domain.UpstreamError
	require.ErrorAs(t, err, &ue)
	assert.Equal(t, http.StatusConflict, ue.Status)
}

func TestCatalogForGame_Fora_Unavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	_, err := New(url, &http.Client{}).CatalogForGame(context.Background(), "g1")
	assert.ErrorIs(t, err, domain.ErrAchievementsUnavailable)
}

func TestUpdateAchievement_PUT_200(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := New(srv.URL, srv.Client()).UpdateAchievement(context.Background(), "g1", "first_win",
		domain.Achievement{Name: "N", EventType: "e", Target: 2})
	require.NoError(t, err)
	assert.Equal(t, http.MethodPut, gotMethod)
	assert.Equal(t, "/admin/games/g1/achievements/first_win", gotPath)
}

func TestUpdateAchievement_404_UpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"achievement_not_found"}`))
	}))
	defer srv.Close()

	err := New(srv.URL, srv.Client()).UpdateAchievement(context.Background(), "g1", "x", domain.Achievement{Name: "N", EventType: "e", Target: 1})
	var ue *domain.UpstreamError
	require.ErrorAs(t, err, &ue)
	assert.Equal(t, http.StatusNotFound, ue.Status)
}

func TestDeleteAchievement_DELETE_204(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	err := New(srv.URL, srv.Client()).DeleteAchievement(context.Background(), "g1", "first_win")
	require.NoError(t, err)
	assert.Equal(t, http.MethodDelete, gotMethod)
	assert.Equal(t, "/admin/games/g1/achievements/first_win", gotPath)
}
