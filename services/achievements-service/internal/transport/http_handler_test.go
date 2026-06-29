package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c130n/game-backend/services/achievements-service/internal/domain"
)

type fakeVerifier struct {
	playerID string
	err      error
}

func (f fakeVerifier) Verify(_ context.Context, _ string) (string, error) {
	return f.playerID, f.err
}

type fakeService struct {
	unlocked    []domain.Achievement
	catalog     []domain.Achievement
	forPlayer   []domain.PlayerAchievement
	recordErr   error
	gotPlayerID string
	gotEvent    domain.Event
}

func (f *fakeService) ListCatalog(_ context.Context) ([]domain.Achievement, error) {
	return f.catalog, nil
}

func (f *fakeService) ListForPlayer(_ context.Context, playerID string) ([]domain.PlayerAchievement, error) {
	f.gotPlayerID = playerID
	return f.forPlayer, nil
}

func (f *fakeService) RecordEvent(_ context.Context, playerID string, event domain.Event) ([]domain.Achievement, error) {
	f.gotPlayerID = playerID
	f.gotEvent = event
	return f.unlocked, f.recordErr
}

func newServer(svc domain.AchievementService, v domain.TokenVerifier) *httptest.Server {
	mux := http.NewServeMux()
	NewHandler(svc, v).Register(mux)
	return httptest.NewServer(mux)
}

func do(t *testing.T, srv *httptest.Server, method, path, token, body string) *http.Response {
	t.Helper()
	var r *http.Request
	var err error
	if body != "" {
		r, err = http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	} else {
		r, err = http.NewRequest(method, srv.URL+path, nil)
	}
	require.NoError(t, err)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := srv.Client().Do(r)
	require.NoError(t, err)
	return resp
}

func TestAuth_SemBearer_401(t *testing.T) {
	srv := newServer(&fakeService{}, fakeVerifier{playerID: "p1"})
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/me/achievements", "", "")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAuth_TokenInvalido_401(t *testing.T) {
	srv := newServer(&fakeService{}, fakeVerifier{err: errors.New("token inválido")})
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/me/achievements", "qualquer", "")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestRecordEvent_Desbloqueia_200(t *testing.T) {
	svc := &fakeService{unlocked: []domain.Achievement{{ID: "first_win", Name: "Primeira Vitória"}}}
	srv := newServer(svc, fakeVerifier{playerID: "player-1"})
	defer srv.Close()

	resp := do(t, srv, http.MethodPost, "/events", "tok", `{"type":"match_won"}`)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Unlocked []achievementDTO `json:"unlocked"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Len(t, body.Unlocked, 1)
	assert.Equal(t, "first_win", body.Unlocked[0].ID)
	assert.Equal(t, "match_won", svc.gotEvent.Type)
}

func TestRecordEvent_PlayerIDVemDoToken_IgnoraCorpo(t *testing.T) {
	svc := &fakeService{}
	srv := newServer(svc, fakeVerifier{playerID: "player-do-token"})
	defer srv.Close()

	// o corpo tenta forjar outro jogador; deve ser ignorado
	resp := do(t, srv, http.MethodPost, "/events", "tok", `{"type":"match_won","player_id":"impostor"}`)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "player-do-token", svc.gotPlayerID)
}

func TestRecordEvent_ErroDominio_MapeiaStatus(t *testing.T) {
	svc := &fakeService{recordErr: domain.ErrInvalidEvent}
	srv := newServer(svc, fakeVerifier{playerID: "p1"})
	defer srv.Close()

	resp := do(t, srv, http.MethodPost, "/events", "tok", `{"type":""}`)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestListForPlayer_UsaPlayerIDDoToken_200(t *testing.T) {
	at := time.Now().UTC()
	svc := &fakeService{forPlayer: []domain.PlayerAchievement{
		{Achievement: domain.Achievement{ID: "first_win", Name: "Primeira Vitória", Target: 1}, Progress: 1, Unlocked: true, UnlockedAt: at},
		{Achievement: domain.Achievement{ID: "ten_wins", Name: "Veterano", Target: 10}, Progress: 7},
	}}
	srv := newServer(svc, fakeVerifier{playerID: "player-9"})
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/me/achievements", "tok", "")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "player-9", svc.gotPlayerID)

	var body struct {
		Achievements []playerAchievementDTO `json:"achievements"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Len(t, body.Achievements, 2)
	// desbloqueada: progress = target, unlocked
	assert.True(t, body.Achievements[0].Unlocked)
	assert.Equal(t, 1, body.Achievements[0].Progress)
	assert.Equal(t, 1, body.Achievements[0].Target)
	assert.NotNil(t, body.Achievements[0].UnlockedAt)
	// em progresso: 7/10, ainda bloqueada
	assert.False(t, body.Achievements[1].Unlocked)
	assert.Equal(t, 7, body.Achievements[1].Progress)
	assert.Equal(t, 10, body.Achievements[1].Target)
	assert.Nil(t, body.Achievements[1].UnlockedAt)
}

// errVerifier sempre falha: prova que as rotas /admin não passam pelo auth.
type errVerifier struct{}

func (errVerifier) Verify(_ context.Context, _ string) (string, error) {
	return "", errors.New("não deveria ser chamado em /admin")
}

func TestAdminListForPlayer_UsaPlayerIDDoPath_200(t *testing.T) {
	at := time.Now().UTC()
	svc := &fakeService{forPlayer: []domain.PlayerAchievement{
		{Achievement: domain.Achievement{ID: "first_win", Name: "Primeira Vitória", Target: 1}, Progress: 1, Unlocked: true, UnlockedAt: at},
	}}
	srv := newServer(svc, errVerifier{}) // sem auth nas rotas /admin
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/admin/players/player-42/achievements", "", "")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "player-42", svc.gotPlayerID, "playerID vem do path, não do JWT")

	var body struct {
		Achievements []playerAchievementDTO `json:"achievements"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Len(t, body.Achievements, 1)
	assert.True(t, body.Achievements[0].Unlocked)
}

func TestAdminListForPlayer_SemProgresso_CatalogoZerado(t *testing.T) {
	svc := &fakeService{forPlayer: []domain.PlayerAchievement{
		{Achievement: domain.Achievement{ID: "ten_wins", Name: "Veterano", Target: 10}, Progress: 0},
	}}
	srv := newServer(svc, errVerifier{})
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/admin/players/novato/achievements", "", "")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Achievements []playerAchievementDTO `json:"achievements"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Len(t, body.Achievements, 1)
	assert.False(t, body.Achievements[0].Unlocked)
	assert.Equal(t, 0, body.Achievements[0].Progress)
	assert.Nil(t, body.Achievements[0].UnlockedAt)
}

func TestAdminListCatalog_200(t *testing.T) {
	svc := &fakeService{catalog: []domain.Achievement{
		{ID: "first_win", Name: "Primeira Vitória", Target: 1},
		{ID: "ten_wins", Name: "Veterano", Target: 10},
	}}
	srv := newServer(svc, errVerifier{})
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/admin/achievements", "", "")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Achievements []achievementDTO `json:"achievements"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Len(t, body.Achievements, 2)
}

func TestAdmin_SemAuthorization_NaoBloqueia(t *testing.T) {
	svc := &fakeService{forPlayer: []domain.PlayerAchievement{}}
	srv := newServer(svc, errVerifier{})
	defer srv.Close()

	// sem header Authorization: o /me responde 401; o /admin responde 200.
	resp := do(t, srv, http.MethodGet, "/admin/players/x/achievements", "", "")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
