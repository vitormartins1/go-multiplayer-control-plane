package identityclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/achievements-service/internal/domain"
)

func fakeIdentity(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Internal-Key") != "ik" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var body string
		switch r.URL.Path {
		case "/admin/players/p-steam/external-identities":
			body = `{"identities":[{"provider":"gog","externalId":"g1"},{"provider":"steam","externalId":"76561198000000042"}]}`
		case "/admin/players/p-senha/external-identities":
			body = `{"identities":[]}`
		case "/admin/players/p-quebra/external-identities":
			w.WriteHeader(http.StatusInternalServerError)
			return
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, err := w.Write([]byte(body))
		assert.NoError(t, err)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestSteamID(t *testing.T) {
	srv := fakeIdentity(t)
	c := New(srv.URL, "ik", nil)
	ctx := context.Background()

	id, err := c.SteamID(ctx, "p-steam")
	require.NoError(t, err)
	assert.Equal(t, "76561198000000042", id)

	_, err = c.SteamID(ctx, "p-senha")
	assert.ErrorIs(t, err, domain.ErrNoSteamAccount, "sem vínculo Steam")
	_, err = c.SteamID(ctx, "p-sumiu")
	assert.ErrorIs(t, err, domain.ErrNoSteamAccount, "conta inexistente: nada a espelhar")

	_, err = c.SteamID(ctx, "p-quebra")
	require.Error(t, err)
	assert.NotErrorIs(t, err, domain.ErrNoSteamAccount, "falha do identity é transitória, não pula")

	_, err = New(srv.URL, "errada", nil).SteamID(ctx, "p-steam")
	assert.Error(t, err)
}
