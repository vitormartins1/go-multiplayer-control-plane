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

	"github.com/vitormartins1/go-multiplayer-control-plane/services/achievements-service/internal/domain"
)

const gid = "game-1"

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
	createErr   error
	updateErr   error
	deleteErr   error
	gotPlayerID string
	gotGameID   string
	gotEvent    domain.Event
	gotCreate   domain.Achievement
	gotUpdate   domain.Achievement
	gotUpdateID string
	gotDeleteID string
}

func (f *fakeService) ListCatalog(_ context.Context, gameID string) ([]domain.Achievement, error) {
	f.gotGameID = gameID
	return f.catalog, nil
}

func (f *fakeService) ListForPlayer(_ context.Context, gameID, playerID string) ([]domain.PlayerAchievement, error) {
	f.gotGameID, f.gotPlayerID = gameID, playerID
	return f.forPlayer, nil
}

func (f *fakeService) RecordEvent(_ context.Context, gameID, playerID string, event domain.Event) ([]domain.Achievement, error) {
	f.gotGameID, f.gotPlayerID, f.gotEvent = gameID, playerID, event
	return f.unlocked, f.recordErr
}

func (f *fakeService) CreateAchievement(_ context.Context, gameID string, a domain.Achievement) (*domain.Achievement, error) {
	f.gotGameID, f.gotCreate = gameID, a
	if f.createErr != nil {
		return nil, f.createErr
	}
	a.GameID = gameID
	return &a, nil
}

func (f *fakeService) UpdateAchievement(_ context.Context, gameID, id string, a domain.Achievement) (*domain.Achievement, error) {
	f.gotGameID, f.gotUpdateID, f.gotUpdate = gameID, id, a
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	a.GameID, a.ID = gameID, id
	return &a, nil
}

func (f *fakeService) DeleteAchievement(_ context.Context, gameID, id string) error {
	f.gotGameID, f.gotDeleteID = gameID, id
	return f.deleteErr
}

func (f *fakeService) ProgressForAchievement(_ context.Context, gameID, _ string) ([]domain.AchievementProgressEntry, error) {
	f.gotGameID = gameID
	return nil, nil
}

// fakeEventRuleService é o fake de domain.EventRuleService — usado direto
// pelos testes das rotas /event-rules; os demais testes (achievements) não se
// importam com ele, então newServer injeta um vazio.
type fakeEventRuleService struct {
	rules       []domain.EventRule
	createErr   error
	updateErr   error
	deleteErr   error
	gotGameID   string
	gotCreate   domain.EventRule
	gotUpdateID string
	gotUpdate   domain.EventRule
	gotDeleteID string
}

func (f *fakeEventRuleService) ListEffectiveRules(_ context.Context, gameID string) ([]domain.EventRule, error) {
	f.gotGameID = gameID
	return f.rules, nil
}

func (f *fakeEventRuleService) CreateEventRule(_ context.Context, gameID string, r domain.EventRule) (*domain.EventRule, error) {
	f.gotGameID, f.gotCreate = gameID, r
	if f.createErr != nil {
		return nil, f.createErr
	}
	gid := gameID
	r.GameID = &gid
	return &r, nil
}

func (f *fakeEventRuleService) UpdateEventRule(_ context.Context, gameID, id string, r domain.EventRule) (*domain.EventRule, error) {
	f.gotGameID, f.gotUpdateID, f.gotUpdate = gameID, id, r
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	gid := gameID
	r.ID = id
	r.GameID = &gid
	r.Archetype = "coop_mission" // o repo preenche via RETURNING; o fake fixa
	return &r, nil
}

func (f *fakeEventRuleService) DeleteEventRule(_ context.Context, gameID, id string) error {
	f.gotGameID, f.gotDeleteID = gameID, id
	return f.deleteErr
}

func newServer(svc domain.AchievementService, v domain.TokenVerifier) *httptest.Server {
	mux := http.NewServeMux()
	NewHandler(svc, &fakeEventRuleService{}, v).Register(mux)
	return httptest.NewServer(mux)
}

func newServerWithRules(svc domain.AchievementService, rules domain.EventRuleService, v domain.TokenVerifier) *httptest.Server {
	mux := http.NewServeMux()
	NewHandler(svc, rules, v).Register(mux)
	return httptest.NewServer(mux)
}

// do faz a request; game != "" injeta o header X-Game-Id (data plane).
func do(t *testing.T, srv *httptest.Server, method, path, token, game, body string) *http.Response {
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
	if game != "" {
		r.Header.Set("X-Game-Id", game)
	}
	resp, err := srv.Client().Do(r)
	require.NoError(t, err)
	return resp
}

func TestAuth_SemBearer_401(t *testing.T) {
	srv := newServer(&fakeService{}, fakeVerifier{playerID: "p1"})
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/me/achievements", "", gid, "")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestRecordEvent_SemGameHeader_400(t *testing.T) {
	srv := newServer(&fakeService{}, fakeVerifier{playerID: "p1"})
	defer srv.Close()

	resp := do(t, srv, http.MethodPost, "/events", "tok", "", `{"type":"match_won"}`) // sem X-Game-Id
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRecordEvent_Desbloqueia_200(t *testing.T) {
	svc := &fakeService{unlocked: []domain.Achievement{{ID: "first_win", Name: "Primeira Vitória"}}}
	srv := newServer(svc, fakeVerifier{playerID: "player-1"})
	defer srv.Close()

	resp := do(t, srv, http.MethodPost, "/events", "tok", gid, `{"type":"match_won"}`)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Unlocked []achievementDTO `json:"unlocked"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Len(t, body.Unlocked, 1)
	assert.Equal(t, "first_win", body.Unlocked[0].ID)
	assert.Equal(t, "match_won", svc.gotEvent.Type)
	assert.Equal(t, gid, svc.gotGameID, "game vem do header")
}

func TestRecordEvent_PlayerIDVemDoToken_IgnoraCorpo(t *testing.T) {
	svc := &fakeService{}
	srv := newServer(svc, fakeVerifier{playerID: "player-do-token"})
	defer srv.Close()

	resp := do(t, srv, http.MethodPost, "/events", "tok", gid, `{"type":"match_won","player_id":"impostor"}`)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "player-do-token", svc.gotPlayerID)
}

func TestListForPlayer_UsaPlayerIDDoToken_200(t *testing.T) {
	at := time.Now().UTC()
	svc := &fakeService{forPlayer: []domain.PlayerAchievement{
		{Achievement: domain.Achievement{ID: "first_win", Name: "Primeira Vitória", Target: 1}, Progress: 1, Unlocked: true, UnlockedAt: at},
	}}
	srv := newServer(svc, fakeVerifier{playerID: "player-9"})
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/me/achievements", "tok", gid, "")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "player-9", svc.gotPlayerID)
	assert.Equal(t, gid, svc.gotGameID)
}

// errVerifier sempre falha: prova que as rotas /admin não passam pelo auth.
type errVerifier struct{}

func (errVerifier) Verify(_ context.Context, _ string) (string, error) {
	return "", errors.New("não deveria ser chamado em /admin")
}

func TestAdminListForPlayer_PorJogoEPlayerDoPath_200(t *testing.T) {
	svc := &fakeService{forPlayer: []domain.PlayerAchievement{
		{Achievement: domain.Achievement{ID: "first_win", Name: "Primeira Vitória", Target: 1}, Progress: 1, Unlocked: true, UnlockedAt: time.Now().UTC()},
	}}
	srv := newServer(svc, errVerifier{})
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/admin/games/game-9/players/player-42/achievements", "", "", "")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "game-9", svc.gotGameID, "gameId vem do path")
	assert.Equal(t, "player-42", svc.gotPlayerID, "playerID vem do path")
}

func TestAdminListCatalog_PorJogo_200(t *testing.T) {
	svc := &fakeService{catalog: []domain.Achievement{
		{ID: "first_win", Name: "Primeira Vitória", Target: 1},
		{ID: "ten_wins", Name: "Veterano", Target: 10},
	}}
	srv := newServer(svc, errVerifier{})
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/admin/games/game-9/achievements", "", "", "")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "game-9", svc.gotGameID)

	var body struct {
		Achievements []achievementDTO `json:"achievements"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Len(t, body.Achievements, 2)
}

func TestAdminCreateAchievement_201(t *testing.T) {
	svc := &fakeService{}
	srv := newServer(svc, errVerifier{})
	defer srv.Close()

	resp := do(t, srv, http.MethodPost, "/admin/games/game-9/achievements", "", "",
		`{"id":"first_blood","name":"Primeiro Sangue","description":"x","eventType":"kill","target":1}`)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Equal(t, "game-9", svc.gotGameID)
	assert.Equal(t, "first_blood", svc.gotCreate.ID)

	var body achievementDTO
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "first_blood", body.ID)
}

func TestAdminCreateAchievement_Invalido_400(t *testing.T) {
	svc := &fakeService{createErr: domain.ErrInvalidAchievement}
	srv := newServer(svc, errVerifier{})
	defer srv.Close()

	resp := do(t, srv, http.MethodPost, "/admin/games/g/achievements", "", "", `{"id":"X","name":"n","eventType":"kill"}`)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestAdminCreateAchievement_JaExiste_409(t *testing.T) {
	svc := &fakeService{createErr: domain.ErrAchievementExists}
	srv := newServer(svc, errVerifier{})
	defer srv.Close()

	resp := do(t, srv, http.MethodPost, "/admin/games/g/achievements", "", "", `{"id":"first_win","name":"P","eventType":"match_won","target":1}`)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestAdminUpdateAchievement_200(t *testing.T) {
	svc := &fakeService{}
	srv := newServer(svc, errVerifier{})
	defer srv.Close()

	resp := do(t, srv, http.MethodPut, "/admin/games/game-9/achievements/first_win", "", "",
		`{"name":"Novo","description":"y","eventType":"match_won","target":5}`)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "game-9", svc.gotGameID)
	assert.Equal(t, "first_win", svc.gotUpdateID)
	assert.Equal(t, 5, svc.gotUpdate.Target)

	var body achievementDTO
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "first_win", body.ID)
	assert.Equal(t, "Novo", body.Name)
}

func TestAdminUpdateAchievement_Inexistente_404(t *testing.T) {
	svc := &fakeService{updateErr: domain.ErrAchievementNotFound}
	srv := newServer(svc, errVerifier{})
	defer srv.Close()

	resp := do(t, srv, http.MethodPut, "/admin/games/g/achievements/fantasma", "", "",
		`{"name":"X","eventType":"e","target":1}`)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestAdminUpdateAchievement_Invalido_400(t *testing.T) {
	svc := &fakeService{updateErr: domain.ErrInvalidAchievement}
	srv := newServer(svc, errVerifier{})
	defer srv.Close()

	resp := do(t, srv, http.MethodPut, "/admin/games/g/achievements/x", "", "",
		`{"name":"","eventType":"","target":0}`)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestAdminDeleteAchievement_204(t *testing.T) {
	svc := &fakeService{}
	srv := newServer(svc, errVerifier{})
	defer srv.Close()

	resp := do(t, srv, http.MethodDelete, "/admin/games/game-9/achievements/first_win", "", "", "")
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Equal(t, "game-9", svc.gotGameID)
	assert.Equal(t, "first_win", svc.gotDeleteID)
}

func TestAdminDeleteAchievement_Inexistente_404(t *testing.T) {
	svc := &fakeService{deleteErr: domain.ErrAchievementNotFound}
	srv := newServer(svc, errVerifier{})
	defer srv.Close()

	resp := do(t, srv, http.MethodDelete, "/admin/games/g/achievements/fantasma", "", "", "")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestAdmin_SemAuthorization_NaoBloqueia(t *testing.T) {
	svc := &fakeService{catalog: []domain.Achievement{}}
	srv := newServer(svc, errVerifier{})
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/admin/games/g/achievements", "", "", "")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAdminRecordEvent_PorJogoEPlayerDoPath_200(t *testing.T) {
	svc := &fakeService{unlocked: []domain.Achievement{{ID: "first_win", Name: "Primeira Vitória"}}}
	srv := newServer(svc, errVerifier{}) // sem JWT (server-authoritative)
	defer srv.Close()

	resp := do(t, srv, http.MethodPost, "/admin/games/game-9/players/player-42/events", "", "", `{"type":"match_won"}`)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "game-9", svc.gotGameID, "gameId do path")
	assert.Equal(t, "player-42", svc.gotPlayerID, "playerID do path (não JWT)")
	assert.Equal(t, "match_won", svc.gotEvent.Type)

	var body struct {
		Unlocked []achievementDTO `json:"unlocked"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Len(t, body.Unlocked, 1)
	assert.Equal(t, "first_win", body.Unlocked[0].ID)
}

func TestAdminRecordEvent_TypeVazio_400(t *testing.T) {
	svc := &fakeService{recordErr: domain.ErrInvalidEvent}
	srv := newServer(svc, errVerifier{})
	defer srv.Close()

	resp := do(t, srv, http.MethodPost, "/admin/games/g/players/p/events", "", "", `{"type":""}`)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestAdminListEventRules_200(t *testing.T) {
	global := domain.EventRule{ID: "r1", GameID: nil, Archetype: "host_world_dropin",
		Match: domain.EqMatch(map[string]string{"event": "JOINED"}), EventTypes: []string{"world_joined"}}
	rules := &fakeEventRuleService{rules: []domain.EventRule{global}}
	srv := newServerWithRules(&fakeService{}, rules, errVerifier{})
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/admin/games/g/event-rules", "", "", "")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "g", rules.gotGameID)

	var body struct {
		Rules []eventRuleDTO `json:"rules"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Len(t, body.Rules, 1)
	assert.Equal(t, "global", body.Rules[0].Scope)
	assert.Equal(t, []string{"world_joined"}, body.Rules[0].EventTypes)
}

func TestAdminCreateEventRule_201(t *testing.T) {
	rules := &fakeEventRuleService{}
	srv := newServerWithRules(&fakeService{}, rules, errVerifier{})
	defer srv.Close()

	resp := do(t, srv, http.MethodPost, "/admin/games/game-9/event-rules", "", "",
		`{"archetype":"battle_royale","match":{"placement":"1"},"eventTypes":["custom_win_badge"]}`)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Equal(t, "game-9", rules.gotGameID)
	assert.Equal(t, "battle_royale", rules.gotCreate.Archetype)
	assert.Equal(t, []string{"custom_win_badge"}, rules.gotCreate.EventTypes)

	var body eventRuleDTO
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "game", body.Scope)
}

func TestAdminCreateEventRule_Invalido_400(t *testing.T) {
	rules := &fakeEventRuleService{createErr: domain.ErrInvalidEventRule}
	srv := newServerWithRules(&fakeService{}, rules, errVerifier{})
	defer srv.Close()

	resp := do(t, srv, http.MethodPost, "/admin/games/g/event-rules", "", "",
		`{"archetype":"arquetipo_invalido","match":{},"eventTypes":["x"]}`)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestAdminUpdateEventRule_200(t *testing.T) {
	rules := &fakeEventRuleService{}
	srv := newServerWithRules(&fakeService{}, rules, errVerifier{})
	defer srv.Close()

	resp := do(t, srv, http.MethodPut, "/admin/games/game-9/event-rules/r7", "", "",
		`{"match":{"stat.revives":{"op":"gte","value":6}},"eventTypes":["mestre_curativo"]}`)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "game-9", rules.gotGameID)
	assert.Equal(t, "r7", rules.gotUpdateID)
	assert.Equal(t, []string{"mestre_curativo"}, rules.gotUpdate.EventTypes)

	var body eventRuleDTO
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "r7", body.ID, "id preservado")
	assert.Equal(t, "game", body.Scope)
}

func TestAdminUpdateEventRule_ArchetypeNoPayload_400(t *testing.T) {
	rules := &fakeEventRuleService{}
	srv := newServerWithRules(&fakeService{}, rules, errVerifier{})
	defer srv.Close()

	// archetype não é editável — decodeStrict rejeita o campo desconhecido.
	resp := do(t, srv, http.MethodPut, "/admin/games/g/event-rules/r7", "", "",
		`{"archetype":"coop_mission","match":{},"eventTypes":["x"]}`)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestAdminUpdateEventRule_GlobalOuInexistente_404(t *testing.T) {
	rules := &fakeEventRuleService{updateErr: domain.ErrEventRuleNotFound}
	srv := newServerWithRules(&fakeService{}, rules, errVerifier{})
	defer srv.Close()

	resp := do(t, srv, http.MethodPut, "/admin/games/g/event-rules/global-1", "", "",
		`{"match":{},"eventTypes":["x"]}`)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "global ou inexistente: mesma resposta")
}

func TestAdminDeleteEventRule_204(t *testing.T) {
	rules := &fakeEventRuleService{}
	srv := newServerWithRules(&fakeService{}, rules, errVerifier{})
	defer srv.Close()

	resp := do(t, srv, http.MethodDelete, "/admin/games/g/event-rules/r1", "", "", "")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Equal(t, "g", rules.gotGameID)
	assert.Equal(t, "r1", rules.gotDeleteID)
}

func TestAdminDeleteEventRule_Inexistente_404(t *testing.T) {
	rules := &fakeEventRuleService{deleteErr: domain.ErrEventRuleNotFound}
	srv := newServerWithRules(&fakeService{}, rules, errVerifier{})
	defer srv.Close()

	resp := do(t, srv, http.MethodDelete, "/admin/games/g/event-rules/global-1", "", "", "")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "global ou inexistente: mesma resposta")
}
