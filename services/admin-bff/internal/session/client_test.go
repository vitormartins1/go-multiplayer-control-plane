package session

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
				"roleSlots":[{"role":"coop","capacity":2,"occupied":1,"occupants":["coop-1"]},{"role":"invader","capacity":1,"occupied":0}]}],
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
	// occupants (session spec 019): quem está em cada papel, repassado 1:1.
	assert.Equal(t, domain.RoleSlot{Role: "coop", Capacity: 2, Occupied: 1, Occupants: []string{"coop-1"}}, s.RoleSlots[0])
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

// Spec 023: status real da Fleet, repassado 1:1 do session-service.
func TestFleetStatus_ParseiaEConverte(t *testing.T) {
	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"cluster":"pi5","fleetExists":true,"desired":2,"current":2,"ready":2,"allocated":1}`))
	}))
	defer upstream.Close()

	got, err := New(upstream.URL, nil).FleetStatus(context.Background(), "capoeira-souls")

	require.NoError(t, err)
	assert.Equal(t, "/admin/fleet-status/capoeira-souls", gotPath)
	assert.Equal(t, domain.FleetStatus{Cluster: "pi5", FleetExists: true, Desired: 2, Current: 2, Ready: 2, Allocated: 1}, got)
}

func TestFleetStatus_UpstreamFora_ErrSessionsUnavailable(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	upstreamURL := upstream.URL
	upstream.Close() // conexão recusada

	_, err := New(upstreamURL, nil).FleetStatus(context.Background(), "capoeira-souls")
	assert.ErrorIs(t, err, domain.ErrSessionsUnavailable)
}

// Spec 018 (session): snapshot do modo, formação e resultado repassados 1:1.
func TestSessionsByGame_DetalheRico_RepassaSnapshotEResultado(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"active": [],
			"closed": [{"sessionId":"s1","matchId":"m1","state":"closed","players":["p1","p3"],
				"openedAt":"2026-07-08T12:00:00Z","closedAt":"2026-07-08T12:01:00Z","closeReason":"result",
				"archetype":"queue_arena","topology":"dedicated","profileHash":"abc123",
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
	assert.Equal(t, "dedicated", s.Topology, "session/031: topologia repassada")
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

// --- spec 025: histórico persistido paginado ---

func TestClosedSessionsByGame_ParseiaPaginaEParticipantes(t *testing.T) {
	var gotPath, gotQuery string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"closed": [{"sessionId":"s0","gameId":"g1","state":"closed","players":["host-1","coop-1"],
				"openedAt":"2026-07-12T10:00:00Z","closedAt":"2026-07-12T10:10:00Z","closeReason":"host_left",
				"origin":"host_world","hostPlayerId":"host-1",
				"participants":[{"playerId":"host-1","role":"host","joinedAt":"2026-07-12T10:00:00Z"},
				                {"playerId":"coop-1","role":"coop","joinedAt":"2026-07-12T10:05:00Z"}]}],
			"nextBefore":"2026-07-12T10:00:00Z"
		}`))
	}))
	defer upstream.Close()

	got, err := New(upstream.URL, nil).ClosedSessionsByGame(context.Background(), "g1", 10, time.Time{})

	require.NoError(t, err)
	assert.Equal(t, "/admin/games/g1/sessions", gotPath)
	assert.Equal(t, "state=closed&limit=10", gotQuery)
	require.Len(t, got.Closed, 1)
	require.Len(t, got.Closed[0].Participants, 2)
	assert.Equal(t, "coop-1", got.Closed[0].Participants[1].PlayerID)
	assert.Equal(t, "coop", got.Closed[0].Participants[1].Role)
	assert.Empty(t, got.Closed[0].Participants[1].Handle, "handle vazio: quem enriquece é o service")
	require.NotNil(t, got.NextBefore)
}

// TestClosedSessionsByGame_RepassaLastEventLastOutcome (spec 025 do
// admin-bff / spec 027 do session-service): passthrough 1:1, sem
// transformação.
func TestClosedSessionsByGame_RepassaLastEventLastOutcome(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"closed": [{"sessionId":"s0","gameId":"g1","state":"closed","players":["host-1","coop-1"],
				"openedAt":"2026-07-12T10:00:00Z","closedAt":"2026-07-12T10:10:00Z","closeReason":"host_killed",
				"origin":"host_world","hostPlayerId":"host-1",
				"participants":[{"playerId":"host-1","role":"host","joinedAt":"2026-07-12T10:00:00Z"},
				                {"playerId":"coop-1","role":"coop","joinedAt":"2026-07-12T10:05:00Z","lastEvent":"DIED","lastOutcome":"FAIL"}]}]
		}`))
	}))
	defer upstream.Close()

	got, err := New(upstream.URL, nil).ClosedSessionsByGame(context.Background(), "g1", 10, time.Time{})

	require.NoError(t, err)
	require.Len(t, got.Closed, 1)
	require.Len(t, got.Closed[0].Participants, 2)
	assert.Empty(t, got.Closed[0].Participants[0].LastEvent, "host sem evento reportado")
	assert.Equal(t, "DIED", got.Closed[0].Participants[1].LastEvent)
	assert.Equal(t, "FAIL", got.Closed[0].Participants[1].LastOutcome)
}

func TestClosedSessionsByGame_ComBefore_RepassaCursorNaQuery(t *testing.T) {
	var gotQuery string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"closed": []}`))
	}))
	defer upstream.Close()

	before := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)
	_, err := New(upstream.URL, nil).ClosedSessionsByGame(context.Background(), "g1", 5, before)
	require.NoError(t, err)
	assert.Equal(t, "state=closed&limit=5&before=2026-07-12T10%3A00%3A00Z", gotQuery)
}

func TestClosedSessionsByGame_503_ErrHistoryUnavailable(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer upstream.Close()

	_, err := New(upstream.URL, nil).ClosedSessionsByGame(context.Background(), "g1", 10, time.Time{})
	assert.ErrorIs(t, err, domain.ErrHistoryUnavailable)
}

func TestClosedSessionsByGame_400_UpstreamError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer upstream.Close()

	_, err := New(upstream.URL, nil).ClosedSessionsByGame(context.Background(), "g1", 10, time.Time{})
	var ue *domain.UpstreamError
	require.ErrorAs(t, err, &ue)
	assert.Equal(t, http.StatusBadRequest, ue.Status)
}

func TestPlayerSessions_ChamaRotaComGameIdNaQuery(t *testing.T) {
	var gotPath, gotQuery string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"closed": []}`))
	}))
	defer upstream.Close()

	_, err := New(upstream.URL, nil).PlayerSessions(context.Background(), "coop-1", "g1", 20, time.Time{})
	require.NoError(t, err)
	assert.Equal(t, "/admin/players/coop-1/sessions", gotPath)
	assert.Equal(t, "gameId=g1&limit=20", gotQuery)
}
