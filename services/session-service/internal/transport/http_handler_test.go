package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/domain"
)

type fakeDirectory struct {
	active, closed []domain.Session
	gotGameID      string

	closedMatchID string          // capturado pelo CloseByAdmin
	closeResult   *domain.Session // devolvido pelo CloseByAdmin (sucesso)
	closeErr      error           // devolvido pelo CloseByAdmin (ex.: ErrSessionNotFound)
}

func (f *fakeDirectory) SessionsByGame(gameID string) ([]domain.Session, []domain.Session) {
	f.gotGameID = gameID
	return f.active, f.closed
}

func (f *fakeDirectory) AllSessions() ([]domain.Session, []domain.Session) {
	return f.active, f.closed
}

func (f *fakeDirectory) CloseByAdmin(_ context.Context, matchID string) (*domain.Session, error) {
	f.closedMatchID = matchID
	return f.closeResult, f.closeErr
}

func newAdminMux(dir *fakeDirectory) *http.ServeMux {
	return newAdminMuxWithFleets(dir, nil, nil, "")
}

// fakeFleetInspector (spec 023) devolve status/erro fixos, sem cluster real.
type fakeFleetInspector struct {
	status  domain.FleetStatus
	err     error
	gotName string
}

func (f *fakeFleetInspector) FleetStatus(_ context.Context, fleetName string) (domain.FleetStatus, error) {
	f.gotName = fleetName
	return f.status, f.err
}

// fakeHistory (spec 025) é o HistoryReader mockado — captura os argumentos
// recebidos e devolve sessions/err fixos.
type fakeHistory struct {
	sessions []domain.Session
	err      error

	gotGameID, gotPlayerID string
	gotLimit               int
	gotBefore              time.Time
	byPlayerCalled         bool
}

func (f *fakeHistory) ClosedByGame(_ context.Context, gameID string, limit int, before time.Time) ([]domain.Session, error) {
	f.gotGameID, f.gotLimit, f.gotBefore = gameID, limit, before
	return f.sessions, f.err
}

func (f *fakeHistory) ClosedByPlayer(_ context.Context, gameID, playerID string, limit int, before time.Time) ([]domain.Session, error) {
	f.byPlayerCalled = true
	f.gotGameID, f.gotPlayerID, f.gotLimit, f.gotBefore = gameID, playerID, limit, before
	return f.sessions, f.err
}

func newAdminMuxWithFleets(dir *fakeDirectory, history HistoryReader, fleets domain.FleetInspector, clusterLabel string) *http.ServeMux {
	mux := http.NewServeMux()
	NewAdminHandler(dir, history, fleets, clusterLabel).Register(mux)
	return mux
}

func newAdminMuxWithHistory(dir *fakeDirectory, history HistoryReader) *http.ServeMux {
	return newAdminMuxWithFleets(dir, history, nil, "")
}

func doGetPage(t *testing.T, mux *http.ServeMux, url string) (*httptest.ResponseRecorder, closedSessionsPage) {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	var body closedSessionsPage
	if rec.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	}
	return rec, body
}

func doGet(t *testing.T, mux *http.ServeMux, url string) (*httptest.ResponseRecorder, sessionsResponse) {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	var body sessionsResponse
	if rec.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	}
	return rec, body
}

func TestAdminSessions_AtivaEFechada_CamposCompletos(t *testing.T) {
	opened := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	closedAt := opened.Add(10 * time.Minute)
	dir := &fakeDirectory{
		active: []domain.Session{{
			ID: "s1", MatchID: "m1", GameID: "g1", Players: []string{"p1", "p2"},
			ServerAddress: "127.0.0.1:7103", State: domain.SessionOpen, OpenedAt: opened,
		}},
		closed: []domain.Session{{
			ID: "s0", MatchID: "m0", GameID: "g1", Players: []string{"p3"},
			ServerAddress: "127.0.0.1:7100", State: domain.SessionClosed,
			OpenedAt: opened.Add(-time.Hour), ClosedAt: closedAt, CloseReason: domain.CloseTTL,
		}},
	}

	rec, body := doGet(t, newAdminMux(dir), "/admin/games/g1/sessions")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "g1", dir.gotGameID)
	require.Len(t, body.Active, 1)
	assert.Equal(t, "s1", body.Active[0].SessionID)
	assert.Equal(t, "open", body.Active[0].State)
	assert.Equal(t, []string{"p1", "p2"}, body.Active[0].Players)
	assert.Equal(t, "127.0.0.1:7103", body.Active[0].ServerAddress)
	assert.Nil(t, body.Active[0].ClosedAt, "aberta não tem closedAt")
	require.Len(t, body.Closed, 1)
	assert.Equal(t, "ttl", body.Closed[0].CloseReason)
	require.NotNil(t, body.Closed[0].ClosedAt)
	assert.Equal(t, closedAt, *body.Closed[0].ClosedAt)
}

func TestAdminSessions_HostWorld_ExpoeOriginHostESlots(t *testing.T) {
	dir := &fakeDirectory{
		active: []domain.Session{{
			ID: "w1", GameID: "hw", Origin: domain.OriginHostWorld, HostPlayerID: "host-1",
			Players: []string{"host-1", "coop-1"}, State: domain.SessionOpen,
			RoleSlots: map[string]*domain.RoleSlot{
				domain.RoleInvader: {Capacity: 1},
				domain.RoleCoop:    {Capacity: 2, Occupants: []string{"coop-1"}},
			},
		}},
	}
	rec, body := doGet(t, newAdminMux(dir), "/admin/games/hw/sessions")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, body.Active, 1)
	s := body.Active[0]
	assert.Equal(t, "host_world", s.Origin)
	assert.Equal(t, "host-1", s.HostPlayerID)
	require.Len(t, s.RoleSlots, 2)
	// ordenado por role: coop antes de invader. Occupants (spec 019) diz quem
	// está em cada papel; slot vazio omite a lista.
	assert.Equal(t, roleSlotDTO{Role: "coop", Capacity: 2, Occupied: 1, Occupants: []string{"coop-1"}}, s.RoleSlots[0])
	assert.Equal(t, roleSlotDTO{Role: "invader", Capacity: 1, Occupied: 0}, s.RoleSlots[1])
}

func TestAdminSessions_QueueArena_SemCamposHostWorld(t *testing.T) {
	dir := &fakeDirectory{
		active: []domain.Session{{ID: "s1", GameID: "g1", Origin: domain.OriginMatch, State: domain.SessionOpen}},
	}
	_, body := doGet(t, newAdminMux(dir), "/admin/games/g1/sessions")
	require.Len(t, body.Active, 1)
	assert.Equal(t, "match", body.Active[0].Origin)
	assert.Empty(t, body.Active[0].HostPlayerID)
	assert.Empty(t, body.Active[0].RoleSlots)
}

func TestAdminSessions_FiltroState(t *testing.T) {
	dir := &fakeDirectory{
		active: []domain.Session{{ID: "s1", GameID: "g1", State: domain.SessionOpen}},
		closed: []domain.Session{{ID: "s0", GameID: "g1", State: domain.SessionClosed}},
	}
	mux := newAdminMux(dir)

	_, open := doGet(t, mux, "/admin/games/g1/sessions?state=open")
	assert.Len(t, open.Active, 1)
	assert.Empty(t, open.Closed)

	rec, _ := doGet(t, mux, "/admin/games/g1/sessions?state=zumbi")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- spec 025: state=closed lê do HistoryStore persistido, paginado ---

func TestAdminSessions_StateClosed_LeDoHistoryStorePaginado(t *testing.T) {
	closedAt := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)
	history := &fakeHistory{sessions: []domain.Session{
		{ID: "s0", GameID: "g1", State: domain.SessionClosed, CloseReason: domain.CloseHostLeft, ClosedAt: closedAt,
			Participants: []domain.Participant{{PlayerID: "host-1", Role: domain.RoleHost, JoinedAt: closedAt}}},
	}}
	mux := newAdminMuxWithHistory(&fakeDirectory{}, history)

	rec, page := doGetPage(t, mux, "/admin/games/g1/sessions?state=closed&limit=10")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "g1", history.gotGameID)
	assert.Equal(t, 11, history.gotLimit, "pede limit+1 para detectar próxima página")
	require.Len(t, page.Closed, 1)
	assert.Equal(t, "s0", page.Closed[0].SessionID)
	require.Len(t, page.Closed[0].Participants, 1)
	assert.Equal(t, "host-1", page.Closed[0].Participants[0].PlayerID)
	assert.Equal(t, []string{"host-1"}, page.Closed[0].Players, "players derivado de participants")
	assert.Nil(t, page.NextBefore, "menos itens que o limite: sem próxima página")
}

func TestAdminSessions_StateClosed_SemHistoryStore_503(t *testing.T) {
	mux := newAdminMux(&fakeDirectory{}) // sem WithHistoryStore
	rec, _ := doGetPage(t, mux, "/admin/games/g1/sessions?state=closed")
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestAdminSessions_StateClosed_ErroNoStore_503(t *testing.T) {
	history := &fakeHistory{err: assert.AnError}
	mux := newAdminMuxWithHistory(&fakeDirectory{}, history)
	rec, _ := doGetPage(t, mux, "/admin/games/g1/sessions?state=closed")
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestAdminSessions_StateClosed_HaMaisPaginas_DevolveNextBefore(t *testing.T) {
	base := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)
	// devolve limit+1 = 3 sessões para limit=2 → sinaliza próxima página.
	history := &fakeHistory{sessions: []domain.Session{
		{ID: "s3", GameID: "g1", ClosedAt: base.Add(2 * time.Minute)},
		{ID: "s2", GameID: "g1", ClosedAt: base.Add(time.Minute)},
		{ID: "s1", GameID: "g1", ClosedAt: base},
	}}
	mux := newAdminMuxWithHistory(&fakeDirectory{}, history)

	rec, page := doGetPage(t, mux, "/admin/games/g1/sessions?state=closed&limit=2")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, page.Closed, 2, "trunca para o limit pedido")
	require.NotNil(t, page.NextBefore)
	assert.Equal(t, base.Add(time.Minute), *page.NextBefore, "cursor = closed_at do último item mantido")
}

func TestAdminSessions_StateClosed_LimitInvalido_400(t *testing.T) {
	mux := newAdminMuxWithHistory(&fakeDirectory{}, &fakeHistory{})
	rec, _ := doGetPage(t, mux, "/admin/games/g1/sessions?state=closed&limit=abc")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdminSessions_StateClosed_BeforeInvalido_400(t *testing.T) {
	mux := newAdminMuxWithHistory(&fakeDirectory{}, &fakeHistory{})
	rec, _ := doGetPage(t, mux, "/admin/games/g1/sessions?state=closed&before=ontem")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- spec 025: aba History do player ---

func TestAdminPlayerSessions_ChamaClosedByPlayer(t *testing.T) {
	history := &fakeHistory{sessions: []domain.Session{{ID: "s1", GameID: "g1"}}}
	mux := newAdminMuxWithHistory(&fakeDirectory{}, history)

	rec, page := doGetPage(t, mux, "/admin/players/p1/sessions?gameId=g1&limit=5")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, history.byPlayerCalled)
	assert.Equal(t, "g1", history.gotGameID)
	assert.Equal(t, "p1", history.gotPlayerID)
	assert.Equal(t, 6, history.gotLimit)
	require.Len(t, page.Closed, 1)
}

func TestAdminPlayerSessions_SemGameId_400(t *testing.T) {
	mux := newAdminMuxWithHistory(&fakeDirectory{}, &fakeHistory{})
	rec, _ := doGetPage(t, mux, "/admin/players/p1/sessions")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdminPlayerSessions_SemHistorico_ListaVaziaSemErro(t *testing.T) {
	mux := newAdminMuxWithHistory(&fakeDirectory{}, &fakeHistory{sessions: nil})
	rec, page := doGetPage(t, mux, "/admin/players/p1/sessions?gameId=g1")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, page.Closed)
	assert.Nil(t, page.NextBefore)
}

func TestAdminSessions_JogoSemSessao_200ComListasVazias(t *testing.T) {
	rec, body := doGet(t, newAdminMux(&fakeDirectory{}), "/admin/games/deserto/sessions")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.NotNil(t, body.Active)
	assert.Empty(t, body.Active)
	assert.Empty(t, body.Closed)
	// contrato do JSON: listas presentes (não null) mesmo vazias.
	assert.Contains(t, rec.Body.String(), `"active":[]`)
	assert.Contains(t, rec.Body.String(), `"closed":[]`)
}

func TestAdminAllSessions_MultiJogo_IncluiGameID(t *testing.T) {
	dir := &fakeDirectory{
		active: []domain.Session{
			{ID: "s1", MatchID: "m1", GameID: "g1", State: domain.SessionOpen, OpenedAt: time.Now()},
			{ID: "s2", MatchID: "m2", GameID: "g2", State: domain.SessionOpen, OpenedAt: time.Now()},
		},
	}
	rec, body := doGet(t, newAdminMux(dir), "/admin/sessions?state=open")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, body.Active, 2)
	games := map[string]bool{body.Active[0].GameID: true, body.Active[1].GameID: true}
	assert.True(t, games["g1"] && games["g2"], "board global traz sessões de jogos distintos com gameId")
}

func TestAdminClose_Ativa_200ComDTO(t *testing.T) {
	dir := &fakeDirectory{closeResult: &domain.Session{
		ID: "s1", MatchID: "m1", GameID: "g1", State: domain.SessionClosed, CloseReason: domain.CloseAdmin,
	}}
	rec := httptest.NewRecorder()
	newAdminMux(dir).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/admin/sessions/m1/close", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "m1", dir.closedMatchID)
	var dto sessionDTO
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &dto))
	assert.Equal(t, "admin", dto.CloseReason)
}

func TestAdminClose_Inexistente_404(t *testing.T) {
	dir := &fakeDirectory{closeErr: domain.ErrSessionNotFound}
	rec := httptest.NewRecorder()
	newAdminMux(dir).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/admin/sessions/nao-existe/close", nil))

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// TestAdminFleetStatus_SemAgones_ClusterVazioSemChamada cobre o caso 2 da spec
// 023: allocator != agones (fleets nil) — nenhuma chamada ao inspector.
func TestAdminFleetStatus_SemAgones_ClusterVazioSemChamada(t *testing.T) {
	mux := newAdminMuxWithFleets(&fakeDirectory{}, nil, nil, "")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/fleet-status/capoeira-souls", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	var dto fleetStatusDTO
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &dto))
	assert.Equal(t, fleetStatusDTO{Cluster: ""}, dto)
}

// TestAdminFleetStatus_AgonesFleetInexistente cobre o caso 3: Agones ativo,
// mas a Fleet não existe NESTE cluster — Cluster preenchido, FleetExists=false.
func TestAdminFleetStatus_AgonesFleetInexistente(t *testing.T) {
	fleets := &fakeFleetInspector{status: domain.FleetStatus{FleetExists: false}}
	mux := newAdminMuxWithFleets(&fakeDirectory{}, nil, fleets, "pi5")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/fleet-status/nao-existe", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	var dto fleetStatusDTO
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &dto))
	assert.Equal(t, fleetStatusDTO{Cluster: "pi5", FleetExists: false}, dto)
	assert.Equal(t, "nao-existe", fleets.gotName)
}

// TestAdminFleetStatus_AgonesFleetExistente cobre o caso 4: Fleet real, réplicas
// reais repassadas junto do cluster.
func TestAdminFleetStatus_AgonesFleetExistente(t *testing.T) {
	fleets := &fakeFleetInspector{status: domain.FleetStatus{
		FleetExists: true, Desired: 2, Current: 2, Ready: 2, Allocated: 1,
	}}
	mux := newAdminMuxWithFleets(&fakeDirectory{}, nil, fleets, "local")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/fleet-status/capoeira-souls", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	var dto fleetStatusDTO
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &dto))
	assert.Equal(t, fleetStatusDTO{
		Cluster: "local", FleetExists: true, Desired: 2, Current: 2, Ready: 2, Allocated: 1,
	}, dto)
}
