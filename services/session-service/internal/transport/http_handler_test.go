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

	"github.com/vitormartins1/go-game-backend/services/session-service/internal/domain"
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
	mux := http.NewServeMux()
	NewAdminHandler(dir).Register(mux)
	return mux
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
	// ordenado por role: coop antes de invader.
	assert.Equal(t, roleSlotDTO{Role: "coop", Capacity: 2, Occupied: 1}, s.RoleSlots[0])
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

	_, closed := doGet(t, mux, "/admin/games/g1/sessions?state=closed")
	assert.Empty(t, closed.Active)
	assert.Len(t, closed.Closed, 1)

	rec, _ := doGet(t, mux, "/admin/games/g1/sessions?state=zumbi")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
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
