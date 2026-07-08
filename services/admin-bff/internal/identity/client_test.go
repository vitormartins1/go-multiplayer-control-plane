package identity

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/admin-bff/internal/domain"
)

func TestListPlayers_MapeiaPaginaERepassaQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/admin/players", r.URL.Path)
		assert.Equal(t, "5", r.URL.Query().Get("limit"))
		assert.Equal(t, "10", r.URL.Query().Get("offset"))
		_, _ = w.Write([]byte(`{"total":14,"players":[
			{"id":"id-1","handle":"Alice","createdAt":"2026-06-29T16:00:00Z"},
			{"id":"id-2","handle":"Bob","createdAt":"2026-06-29T15:00:00Z"}
		]}`))
	}))
	defer srv.Close()

	page, err := New(srv.URL, srv.Client()).ListPlayers(context.Background(), 5, 10)
	require.NoError(t, err)
	assert.Equal(t, 14, page.Total)
	require.Len(t, page.Players, 2)
	assert.Equal(t, "Alice", page.Players[0].Handle)
	assert.False(t, page.Players[0].CreatedAt.IsZero())
}

func TestListPlayers_IdentityFora_ErrIdentityUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	_, err := New(url, &http.Client{}).ListPlayers(context.Background(), 50, 0)
	assert.ErrorIs(t, err, domain.ErrIdentityUnavailable)
}

func TestGetPlayer_MapeiaJogador(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/admin/players/id-1", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":"id-1","handle":"Alice","createdAt":"2026-06-29T16:00:00Z"}`))
	}))
	defer srv.Close()

	p, err := New(srv.URL, srv.Client()).GetPlayer(context.Background(), "id-1")
	require.NoError(t, err)
	assert.Equal(t, "Alice", p.Handle)
	assert.False(t, p.CreatedAt.IsZero())
}

func TestGetPlayer_404_ErrPlayerNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := New(srv.URL, srv.Client()).GetPlayer(context.Background(), "nope")
	assert.ErrorIs(t, err, domain.ErrPlayerNotFound)
}

func TestGetPlayer_IdentityFora_ErrIdentityUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	_, err := New(url, &http.Client{}).GetPlayer(context.Background(), "id-1")
	assert.ErrorIs(t, err, domain.ErrIdentityUnavailable)
}

func TestListPlayers_HTTP500_ErrIdentityUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := New(srv.URL, srv.Client()).ListPlayers(context.Background(), 50, 0)
	assert.ErrorIs(t, err, domain.ErrIdentityUnavailable)
}
