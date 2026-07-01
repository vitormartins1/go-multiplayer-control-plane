package cloudsave

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

func TestForPlayer_200(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"contentType":"application/json","size":9,"updatedAt":"2026-06-30T12:00:00Z","blob":"eyJocCI6NTB9"}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL, srv.Client()).ForPlayer(context.Background(), "game-1", "player-7")
	require.NoError(t, err)
	assert.Equal(t, "/admin/games/game-1/players/player-7/save", gotPath)
	assert.Equal(t, "application/json", got.ContentType)
	assert.Equal(t, 9, got.Size)
	assert.False(t, got.UpdatedAt.IsZero())
	assert.Equal(t, "eyJocCI6NTB9", got.Blob)
}

func TestForPlayer_404_UpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not_found"}`))
	}))
	defer srv.Close()

	_, err := New(srv.URL, srv.Client()).ForPlayer(context.Background(), "game-1", "p1")
	var ue *domain.UpstreamError
	require.ErrorAs(t, err, &ue)
	assert.Equal(t, http.StatusNotFound, ue.Status)
	assert.Equal(t, "not_found", ue.Code)
}

func TestForPlayer_Fora_ErrCloudSaveUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := New(srv.URL, srv.Client()).ForPlayer(context.Background(), "game-1", "p1")
	assert.True(t, errors.Is(err, domain.ErrCloudSaveUnavailable))
}
