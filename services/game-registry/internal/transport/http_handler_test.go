package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-game-backend/services/game-registry/internal/domain"
)

type mockService struct{ mock.Mock }

func (m *mockService) Register(ctx context.Context, slug, name string) (*domain.Game, error) {
	args := m.Called(ctx, slug, name)
	g, _ := args.Get(0).(*domain.Game)
	return g, args.Error(1)
}
func (m *mockService) List(ctx context.Context) ([]domain.Game, error) {
	args := m.Called(ctx)
	games, _ := args.Get(0).([]domain.Game)
	return games, args.Error(1)
}
func (m *mockService) Get(ctx context.Context, id string) (*domain.Game, error) {
	args := m.Called(ctx, id)
	g, _ := args.Get(0).(*domain.Game)
	return g, args.Error(1)
}

func (m *mockService) Rename(ctx context.Context, id, name string) (*domain.Game, error) {
	args := m.Called(ctx, id, name)
	g, _ := args.Get(0).(*domain.Game)
	return g, args.Error(1)
}
func (m *mockService) SetMultiplayerProfile(ctx context.Context, gameID string, p domain.MultiplayerProfile) (*domain.MultiplayerProfile, error) {
	args := m.Called(ctx, gameID, p)
	res, _ := args.Get(0).(*domain.MultiplayerProfile)
	return res, args.Error(1)
}
func (m *mockService) GetMultiplayerProfile(ctx context.Context, gameID string) (*domain.MultiplayerProfile, error) {
	args := m.Called(ctx, gameID)
	res, _ := args.Get(0).(*domain.MultiplayerProfile)
	return res, args.Error(1)
}

type mockPresence struct{ mock.Mock }

func (m *mockPresence) Mark(ctx context.Context, gameID, playerID, handle string) error {
	return m.Called(ctx, gameID, playerID, handle).Error(0)
}
func (m *mockPresence) PlayersOf(ctx context.Context, gameID string) ([]domain.Presence, error) {
	args := m.Called(ctx, gameID)
	ps, _ := args.Get(0).([]domain.Presence)
	return ps, args.Error(1)
}
func (m *mockPresence) GamesOf(ctx context.Context, playerID string) ([]domain.Game, error) {
	args := m.Called(ctx, playerID)
	gs, _ := args.Get(0).([]domain.Game)
	return gs, args.Error(1)
}

func newServer(svc domain.GameService) http.Handler {
	return newServerWith(svc, new(mockPresence))
}

func newServerWith(svc domain.GameService, presence domain.PresenceService) http.Handler {
	mux := http.NewServeMux()
	NewHandler(svc, presence).Register(mux)
	return mux
}

func do(srv http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	return rr
}

func TestCreate_Valido_201(t *testing.T) {
	svc := new(mockService)
	g := &domain.Game{ID: "id-1", Slug: "tetris", Name: "Tetris"}
	svc.On("Register", mock.Anything, "tetris", "Tetris").Return(g, nil)

	rr := do(newServer(svc), http.MethodPost, "/games", `{"slug":"tetris","name":"Tetris"}`)
	require.Equal(t, http.StatusCreated, rr.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "id-1", resp["id"])
	assert.Equal(t, "tetris", resp["slug"])
}

func TestCreate_ExpoeMetadadosDeCapa(t *testing.T) {
	svc := new(mockService)
	g := &domain.Game{
		ID: "id-1", Slug: "tetris", Name: "Tetris",
		ImageSource: "generated", GeneratedImageSeed: "tetris", GeneratedImageStyle: "shapes",
	}
	svc.On("Register", mock.Anything, "tetris", "Tetris").Return(g, nil)

	rr := do(newServer(svc), http.MethodPost, "/games", `{"slug":"tetris","name":"Tetris"}`)
	require.Equal(t, http.StatusCreated, rr.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "generated", resp["imageSource"])
	assert.Equal(t, "tetris", resp["generatedImageSeed"])
	assert.Equal(t, "shapes", resp["generatedImageStyle"])
	assert.Equal(t, "", resp["image"])
}

func TestCreate_SlugInvalido_400(t *testing.T) {
	svc := new(mockService)
	svc.On("Register", mock.Anything, mock.Anything, mock.Anything).Return(nil, domain.ErrInvalidSlug)
	rr := do(newServer(svc), http.MethodPost, "/games", `{"slug":"X","name":"n"}`)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreate_NameInvalido_400(t *testing.T) {
	svc := new(mockService)
	svc.On("Register", mock.Anything, mock.Anything, mock.Anything).Return(nil, domain.ErrInvalidName)
	rr := do(newServer(svc), http.MethodPost, "/games", `{"slug":"tetris","name":""}`)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreate_SlugTaken_409(t *testing.T) {
	svc := new(mockService)
	svc.On("Register", mock.Anything, mock.Anything, mock.Anything).Return(nil, domain.ErrSlugTaken)
	rr := do(newServer(svc), http.MethodPost, "/games", `{"slug":"tetris","name":"Tetris"}`)
	assert.Equal(t, http.StatusConflict, rr.Code)
}

func TestList_200(t *testing.T) {
	svc := new(mockService)
	svc.On("List", mock.Anything).Return([]domain.Game{{ID: "a", Slug: "arkanoid", Name: "Arkanoid"}}, nil)
	rr := do(newServer(svc), http.MethodGet, "/games", "")
	require.Equal(t, http.StatusOK, rr.Code)

	var resp []map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.Len(t, resp, 1)
	assert.Equal(t, "arkanoid", resp[0]["slug"])
}

func TestGet_200(t *testing.T) {
	svc := new(mockService)
	svc.On("Get", mock.Anything, "id-1").Return(&domain.Game{ID: "id-1", Slug: "tetris", Name: "Tetris"}, nil)
	rr := do(newServer(svc), http.MethodGet, "/games/id-1", "")
	require.Equal(t, http.StatusOK, rr.Code)
}

func TestGet_Inexistente_404(t *testing.T) {
	svc := new(mockService)
	svc.On("Get", mock.Anything, "nope").Return(nil, domain.ErrGameNotFound)
	rr := do(newServer(svc), http.MethodGet, "/games/nope", "")
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestMarkPresence_204(t *testing.T) {
	pres := new(mockPresence)
	pres.On("Mark", mock.Anything, "g1", "p1", "Alice").Return(nil)
	rr := do(newServerWith(new(mockService), pres), http.MethodPut, "/games/g1/players/p1", `{"handle":"Alice"}`)
	assert.Equal(t, http.StatusNoContent, rr.Code)
	pres.AssertExpectations(t)
}

func TestMarkPresence_JogoInexistente_404(t *testing.T) {
	pres := new(mockPresence)
	pres.On("Mark", mock.Anything, "nope", "p1", "Alice").Return(domain.ErrGameNotFound)
	rr := do(newServerWith(new(mockService), pres), http.MethodPut, "/games/nope/players/p1", `{"handle":"Alice"}`)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestPlayerGames_200(t *testing.T) {
	pres := new(mockPresence)
	pres.On("GamesOf", mock.Anything, "p1").Return([]domain.Game{
		{ID: "g1", Slug: "tetris", Name: "Tetris", GeneratedImageSeed: "tetris"},
	}, nil)
	rr := do(newServerWith(new(mockService), pres), http.MethodGet, "/players/p1/games", "")
	require.Equal(t, http.StatusOK, rr.Code)

	var resp []map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.Len(t, resp, 1)
	assert.Equal(t, "tetris", resp[0]["slug"])
	assert.Equal(t, "tetris", resp[0]["generatedImageSeed"])
}

func TestListPlayers_200(t *testing.T) {
	pres := new(mockPresence)
	pres.On("PlayersOf", mock.Anything, "g1").Return([]domain.Presence{{PlayerID: "p1", Handle: "Alice"}}, nil)
	rr := do(newServerWith(new(mockService), pres), http.MethodGet, "/games/g1/players", "")
	require.Equal(t, http.StatusOK, rr.Code)

	var resp []map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.Len(t, resp, 1)
	assert.Equal(t, "p1", resp[0]["playerId"])
	assert.Equal(t, "Alice", resp[0]["handle"])
}

func TestRename_200_AtualizaNome(t *testing.T) {
	svc := new(mockService)
	svc.On("Rename", mock.Anything, "id-1", "Novo").Return(&domain.Game{ID: "id-1", Slug: "orb", Name: "Novo"}, nil)

	rr := do(newServer(svc), http.MethodPut, "/games/id-1", `{"name":"Novo"}`)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"name":"Novo"`)
	svc.AssertExpectations(t)
}

func TestRename_NomeInvalido_400(t *testing.T) {
	svc := new(mockService)
	svc.On("Rename", mock.Anything, "id-1", "").Return((*domain.Game)(nil), domain.ErrInvalidName)
	rr := do(newServer(svc), http.MethodPut, "/games/id-1", `{"name":""}`)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestRename_Inexistente_404(t *testing.T) {
	svc := new(mockService)
	svc.On("Rename", mock.Anything, "nao", "X").Return((*domain.Game)(nil), domain.ErrGameNotFound)
	rr := do(newServer(svc), http.MethodPut, "/games/nao", `{"name":"X"}`)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// --- perfil de multiplayer (spec 006) ---

func ffaProfile() *domain.MultiplayerProfile {
	return &domain.MultiplayerProfile{
		Archetype: domain.ArchetypeQueueArena, Topology: domain.TopologyDedicated,
		QueueArena: &domain.QueueArenaParams{
			Structure: "ffa", PlayerCount: 8,
			WinCondition: "first_to_score", ScoreTarget: 25, MatchStructure: "single",
		},
	}
}

const validProfileBody = `{"archetype":"queue_arena","topology":"dedicated","params":{"structure":"ffa","playerCount":8,"winCondition":"first_to_score","scoreTarget":25,"matchStructure":"single"}}`

func TestSetProfile_Valido_200_MapeiaParams(t *testing.T) {
	svc := new(mockService)
	svc.On("SetMultiplayerProfile", mock.Anything, "id-1", mock.MatchedBy(func(p domain.MultiplayerProfile) bool {
		return p.Archetype == "queue_arena" && p.Topology == "dedicated" &&
			p.QueueArena != nil && p.QueueArena.Structure == "ffa" && p.QueueArena.PlayerCount == 8
	})).Return(ffaProfile(), nil)

	rr := do(newServer(svc), http.MethodPut, "/games/id-1/multiplayer-profile", validProfileBody)
	require.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "queue_arena", resp["archetype"])
	svc.AssertExpectations(t)
}

func brProfile() *domain.MultiplayerProfile {
	return &domain.MultiplayerProfile{
		Archetype: domain.ArchetypeBattleRoyale, Topology: domain.TopologyDedicated,
		BattleRoyale: &domain.BattleRoyaleParams{
			LobbySize: 60, TeamSize: 4, RespawnMode: domain.RespawnArena1v1, ScoringStats: []string{"kills"},
		},
	}
}

const validBRProfileBody = `{"archetype":"battle_royale","topology":"dedicated","params":{"lobbySize":60,"teamSize":4,"respawnMode":"arena_1v1","scoringStats":["kills"]}}`

func TestSetProfile_BattleRoyale_200_MapeiaParams(t *testing.T) {
	svc := new(mockService)
	svc.On("SetMultiplayerProfile", mock.Anything, "id-1", mock.MatchedBy(func(p domain.MultiplayerProfile) bool {
		return p.Archetype == "battle_royale" && p.Topology == "dedicated" &&
			p.BattleRoyale != nil && p.BattleRoyale.LobbySize == 60 && p.BattleRoyale.TeamSize == 4 &&
			p.BattleRoyale.RespawnMode == "arena_1v1"
	})).Return(brProfile(), nil)

	rr := do(newServer(svc), http.MethodPut, "/games/id-1/multiplayer-profile", validBRProfileBody)
	require.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "battle_royale", resp["archetype"])
	params, ok := resp["params"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "arena_1v1", params["respawnMode"])
	svc.AssertExpectations(t)
}

func TestGet_IncluiMultiplayerProfile_BattleRoyale(t *testing.T) {
	svc := new(mockService)
	svc.On("Get", mock.Anything, "id-1").
		Return(&domain.Game{ID: "id-1", Slug: "br", Name: "BR", MultiplayerProfile: brProfile()}, nil)
	rr := do(newServer(svc), http.MethodGet, "/games/id-1", "")
	require.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	prof, ok := resp["multiplayerProfile"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "battle_royale", prof["archetype"])
	params, ok := prof["params"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(4), params["teamSize"])
}

func cmProfile() *domain.MultiplayerProfile {
	return &domain.MultiplayerProfile{
		Archetype: domain.ArchetypeCoopMission, Topology: domain.TopologyDedicated,
		CoopMission: &domain.CoopMissionParams{
			TeamSize: 4, Missions: []string{"salvage_op"}, Difficulties: []string{"hazard_5"},
			BotfillEnabled: true, ScoringStats: []string{"kills", "revives"},
		},
	}
}

const validCMProfileBody = `{"archetype":"coop_mission","topology":"dedicated","params":{"teamSize":4,"missions":["salvage_op"],"difficulties":["hazard_5"],"botfillEnabled":true,"scoringStats":["kills","revives"]}}`

func TestSetProfile_CoopMission_200_MapeiaParams(t *testing.T) {
	svc := new(mockService)
	svc.On("SetMultiplayerProfile", mock.Anything, "id-1", mock.MatchedBy(func(p domain.MultiplayerProfile) bool {
		return p.Archetype == "coop_mission" && p.Topology == "dedicated" &&
			p.CoopMission != nil && p.CoopMission.TeamSize == 4 && p.CoopMission.BotfillEnabled &&
			len(p.CoopMission.Missions) == 1 && p.CoopMission.Missions[0] == "salvage_op"
	})).Return(cmProfile(), nil)

	rr := do(newServer(svc), http.MethodPut, "/games/id-1/multiplayer-profile", validCMProfileBody)
	require.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "coop_mission", resp["archetype"])
	params, ok := resp["params"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(4), params["teamSize"])
	assert.Equal(t, true, params["botfillEnabled"])
	svc.AssertExpectations(t)
}

func TestGet_IncluiMultiplayerProfile_CoopMission(t *testing.T) {
	svc := new(mockService)
	svc.On("Get", mock.Anything, "id-1").
		Return(&domain.Game{ID: "id-1", Slug: "cm", Name: "CM", MultiplayerProfile: cmProfile()}, nil)
	rr := do(newServer(svc), http.MethodGet, "/games/id-1", "")
	require.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	prof, ok := resp["multiplayerProfile"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "coop_mission", prof["archetype"])
	params, ok := prof["params"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(4), params["teamSize"])
}

func TestSetProfile_BodyInvalido_400(t *testing.T) {
	rr := do(newServer(new(mockService)), http.MethodPut, "/games/id-1/multiplayer-profile", `{`)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestSetProfile_ArchetypeNaoSuportado_400(t *testing.T) {
	svc := new(mockService)
	svc.On("SetMultiplayerProfile", mock.Anything, mock.Anything, mock.Anything).
		Return((*domain.MultiplayerProfile)(nil), domain.ErrUnsupportedArchetype)
	rr := do(newServer(svc), http.MethodPut, "/games/id-1/multiplayer-profile", validProfileBody)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "unsupported_archetype")
}

func TestSetProfile_ParamsInvalidos_400(t *testing.T) {
	svc := new(mockService)
	svc.On("SetMultiplayerProfile", mock.Anything, mock.Anything, mock.Anything).
		Return((*domain.MultiplayerProfile)(nil), domain.ErrInvalidProfileParams)
	rr := do(newServer(svc), http.MethodPut, "/games/id-1/multiplayer-profile", validProfileBody)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestSetProfile_JogoInexistente_404(t *testing.T) {
	svc := new(mockService)
	svc.On("SetMultiplayerProfile", mock.Anything, mock.Anything, mock.Anything).
		Return((*domain.MultiplayerProfile)(nil), domain.ErrGameNotFound)
	rr := do(newServer(svc), http.MethodPut, "/games/nao/multiplayer-profile", validProfileBody)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestGetProfile_Existe_200(t *testing.T) {
	svc := new(mockService)
	svc.On("GetMultiplayerProfile", mock.Anything, "id-1").Return(ffaProfile(), nil)
	rr := do(newServer(svc), http.MethodGet, "/games/id-1/multiplayer-profile", "")
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"archetype":"queue_arena"`)
}

func TestGetProfile_Ausente_404(t *testing.T) {
	svc := new(mockService)
	svc.On("GetMultiplayerProfile", mock.Anything, "id-1").
		Return((*domain.MultiplayerProfile)(nil), domain.ErrProfileNotFound)
	rr := do(newServer(svc), http.MethodGet, "/games/id-1/multiplayer-profile", "")
	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.Contains(t, rr.Body.String(), "profile_not_found")
}

func TestGet_IncluiMultiplayerProfile(t *testing.T) {
	svc := new(mockService)
	svc.On("Get", mock.Anything, "id-1").
		Return(&domain.Game{ID: "id-1", Slug: "arena", Name: "Arena", MultiplayerProfile: ffaProfile()}, nil)
	rr := do(newServer(svc), http.MethodGet, "/games/id-1", "")
	require.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	prof, ok := resp["multiplayerProfile"].(map[string]any)
	require.True(t, ok, "multiplayerProfile presente como objeto")
	assert.Equal(t, "queue_arena", prof["archetype"])
}

func TestGet_SemPerfil_MultiplayerProfileNull(t *testing.T) {
	svc := new(mockService)
	svc.On("Get", mock.Anything, "id-1").Return(&domain.Game{ID: "id-1", Slug: "arena", Name: "Arena"}, nil)
	rr := do(newServer(svc), http.MethodGet, "/games/id-1", "")
	require.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	val, ok := resp["multiplayerProfile"]
	require.True(t, ok, "chave presente")
	assert.Nil(t, val, "valor null quando sem perfil")
}

func TestList_NaoIncluiMultiplayerProfile(t *testing.T) {
	svc := new(mockService)
	svc.On("List", mock.Anything).Return([]domain.Game{{ID: "id-1", Slug: "arena", Name: "Arena"}}, nil)
	rr := do(newServer(svc), http.MethodGet, "/games", "")
	require.Equal(t, http.StatusOK, rr.Code)

	var arr []map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &arr))
	require.Len(t, arr, 1)
	_, ok := arr[0]["multiplayerProfile"]
	assert.False(t, ok, "lista não infla com o perfil")
}
