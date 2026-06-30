package playerdata

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

func TestForPlayer_200(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"data":[{"namespace":"inventory","key":"gold","value":100},{"namespace":"stats","key":"kills","value":{"total":10}}]}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL, srv.Client()).ForPlayer(context.Background(), "game-1", "player-7")
	require.NoError(t, err)
	assert.Equal(t, "/admin/games/game-1/players/player-7/data", gotPath)
	require.Len(t, got, 2)
	assert.Equal(t, "inventory", got[0].Namespace)
	assert.Equal(t, "gold", got[0].Key)
	assert.JSONEq(t, `100`, string(got[0].Value))
	assert.JSONEq(t, `{"total":10}`, string(got[1].Value))
}

func TestForPlayer_Vazio_200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL, srv.Client()).ForPlayer(context.Background(), "game-1", "p1")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestForPlayer_Fora_ErrPlayerDataUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := New(srv.URL, srv.Client()).ForPlayer(context.Background(), "game-1", "p1")
	assert.True(t, errors.Is(err, domain.ErrPlayerDataUnavailable))
}

func TestForPlayer_401_UpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer srv.Close()

	_, err := New(srv.URL, srv.Client()).ForPlayer(context.Background(), "game-1", "p1")
	var ue *domain.UpstreamError
	require.ErrorAs(t, err, &ue)
	assert.Equal(t, http.StatusUnauthorized, ue.Status)
	assert.Equal(t, "unauthorized", ue.Code)
}
