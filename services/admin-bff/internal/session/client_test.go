package session

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/admin-bff/internal/domain"
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

func TestSessionsByGame_HostWorld_RepassaOriginESlots(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"active": [{"sessionId":"w1","state":"open","players":["host-1","coop-1"],"origin":"host_world","hostPlayerId":"host-1",
				"roleSlots":[{"role":"coop","capacity":2,"occupied":1},{"role":"invader","capacity":1,"occupied":0}]}],
			"closed": []
		}`))
	}))
	defer upstream.Close()

	got, err := New(upstream.URL, nil).SessionsByGame(context.Background(), "hw", "")
	require.NoError(t, err)
	require.Len(t, got.Active, 1)
	s := got.Active[0]
	assert.Equal(t, "host_world", s.Origin)
	assert.Equal(t, "host-1", s.HostPlayerID)
	require.Len(t, s.RoleSlots, 2)
	assert.Equal(t, domain.RoleSlot{Role: "coop", Capacity: 2, Occupied: 1}, s.RoleSlots[0])
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

func TestAllSessions_ParseiaGameIDEFiltra(t *testing.T) {
	var gotPath, gotQuery string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"active": [{"sessionId":"s1","matchId":"m1","gameId":"g1","state":"open","players":["p1"],"serverAddress":"127.0.0.1:7103","openedAt":"2026-07-03T12:00:00Z"}],
			"closed": []
		}`))
	}))
	defer upstream.Close()

	got, err := New(upstream.URL, nil).AllSessions(context.Background(), "open")

	require.NoError(t, err)
	assert.Equal(t, "/admin/sessions", gotPath)
	assert.Equal(t, "state=open", gotQuery)
	require.Len(t, got.Active, 1)
	assert.Equal(t, "g1", got.Active[0].GameID, "board global carrega o gameId")
}

func TestCloseSession_Ok_ParseiaSessao(t *testing.T) {
	var gotMethod, gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sessionId":"s1","matchId":"m1","gameId":"g1","state":"closed","players":["p1"],"closeReason":"admin","openedAt":"2026-07-03T12:00:00Z"}`))
	}))
	defer upstream.Close()

	got, err := New(upstream.URL, nil).CloseSession(context.Background(), "m1")

	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "/admin/sessions/m1/close", gotPath)
	assert.Equal(t, "admin", got.CloseReason)
}

func TestCloseSession_404_UpstreamError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer upstream.Close()

	_, err := New(upstream.URL, nil).CloseSession(context.Background(), "nao-existe")

	var ue *domain.UpstreamError
	require.ErrorAs(t, err, &ue)
	assert.Equal(t, http.StatusNotFound, ue.Status)
}

// Spec 018 (session): snapshot do modo, formação e resultado repassados 1:1.
func TestSessionsByGame_DetalheRico_RepassaSnapshotEResultado(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"active": [],
			"closed": [{"sessionId":"s1","matchId":"m1","state":"closed","players":["p1","p3"],
				"openedAt":"2026-07-08T12:00:00Z","closedAt":"2026-07-08T12:01:00Z","closeReason":"result",
				"archetype":"queue_arena","profileHash":"abc123",
				"modeParams":{"structure":"teams","scoreTarget":5},
				"teams":[{"teamId":"team-1","players":["p1"]},{"teamId":"team-2","players":["p3"]}],
				"result":{"kind":"win_loss_draw","winLossDraw":[
					{"playerId":"p1","teamId":"team-1","outcome":"win","score":4,"stats":{"orbs":4}},
					{"playerId":"p3","teamId":"team-2","outcome":"loss","score":1}]}}]
		}`))
	}))
	defer upstream.Close()

	got, err := New(upstream.URL, nil).SessionsByGame(context.Background(), "g1", "")

	require.NoError(t, err)
	require.Len(t, got.Closed, 1)
	s := got.Closed[0]
	assert.Equal(t, "queue_arena", s.Archetype)
	assert.Equal(t, "abc123", s.ProfileHash)
	assert.JSONEq(t, `{"structure":"teams","scoreTarget":5}`, string(s.ModeParams))
	require.Len(t, s.Teams, 2)
	assert.Equal(t, domain.SessionTeam{TeamID: "team-1", Players: []string{"p1"}}, s.Teams[0])
	require.NotNil(t, s.Result)
	assert.Equal(t, "win_loss_draw", s.Result.Kind)
	require.Len(t, s.Result.WinLossDraw, 2)
	assert.Equal(t, "win", s.Result.WinLossDraw[0].Outcome)
	assert.Equal(t, int64(4), s.Result.WinLossDraw[0].Stats["orbs"])
}
