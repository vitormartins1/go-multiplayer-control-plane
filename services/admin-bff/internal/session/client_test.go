package session

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-game-backend/services/admin-bff/internal/domain"
)

func TestSessionsByGame_ParseiaEConverte(t *testing.T) {
	var gotPath, gotQuery string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"active": [{"sessionId":"s1","matchId":"m1","state":"open","players":["p1","p2"],"serverAddress":"127.0.0.1:7103","openedAt":"2026-07-03T12:00:00Z"}],
			"closed": [{"sessionId":"s0","matchId":"m0","state":"closed","players":["p3"],"serverAddress":"127.0.0.1:7100","openedAt":"2026-07-03T11:00:00Z","closedAt":"2026-07-03T11:10:00Z","closeReason":"ttl"}]
		}`))
	}))
	defer upstream.Close()

	got, err := New(upstream.URL, nil).SessionsByGame(context.Background(), "g1", "open")

	require.NoError(t, err)
	assert.Equal(t, "/admin/games/g1/sessions", gotPath)
	assert.Equal(t, "state=open", gotQuery, "filtro repassado")
	require.Len(t, got.Active, 1)
	assert.Equal(t, "m1", got.Active[0].MatchID)
	assert.Equal(t, []domain.SessionPlayer{{ID: "p1"}, {ID: "p2"}}, got.Active[0].Players, "handles vazios: quem enriquece é o service")
	require.Len(t, got.Closed, 1)
	assert.Equal(t, "ttl", got.Closed[0].CloseReason)
	require.NotNil(t, got.Closed[0].ClosedAt)
}

func TestSessionsByGame_UpstreamFora_ErrSessionsUnavailable(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	upstreamURL := upstream.URL
	upstream.Close() // conexão recusada

	_, err := New(upstreamURL, nil).SessionsByGame(context.Background(), "g1", "")
	assert.ErrorIs(t, err, domain.ErrSessionsUnavailable)
}

func TestSessionsByGame_Status500_ErrSessionsUnavailable(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer upstream.Close()

	_, err := New(upstream.URL, nil).SessionsByGame(context.Background(), "g1", "")
	assert.ErrorIs(t, err, domain.ErrSessionsUnavailable)
}
