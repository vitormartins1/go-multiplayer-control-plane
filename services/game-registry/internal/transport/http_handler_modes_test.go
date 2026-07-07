package transport

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/game-registry/internal/domain"
)

// Testes das rotas de modos nomeados (ADR-0024, spec 009). mockService/do vivem
// em http_handler_test.go (mesmo package).

func sampleMode() *domain.MultiplayerMode {
	return &domain.MultiplayerMode{
		ModeID: "tdm", DisplayName: "Team Deathmatch",
		Profile: domain.MultiplayerProfile{
			Archetype: domain.ArchetypeQueueArena, Topology: domain.TopologyDedicated,
			QueueArena: &domain.QueueArenaParams{
				Structure: "teams", TeamCount: 2, TeamSize: 4,
				WinCondition: "first_to_score", ScoreTarget: 50, MatchStructure: "single",
			},
		},
	}
}

func TestSetMode_Valido_200ComModoAchatado(t *testing.T) {
	svc := new(mockService)
	svc.On("SetMode", mock.Anything, "g1", "tdm", "Team Deathmatch", mock.Anything).
		Return(sampleMode(), nil)

	body := `{"displayName":"Team Deathmatch","archetype":"queue_arena","topology":"dedicated",` +
		`"params":{"structure":"teams","teamCount":2,"teamSize":4,"winCondition":"first_to_score","scoreTarget":50,"matchStructure":"single","allowDraw":false}}`
	rr := do(newServer(svc), http.MethodPut, "/games/g1/multiplayer-profiles/tdm", body)
	require.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "tdm", resp["modeId"])
	assert.Equal(t, "Team Deathmatch", resp["displayName"])
	assert.Equal(t, "queue_arena", resp["archetype"], "embed do profileDTO achata archetype no topo")
	assert.NotNil(t, resp["params"])
}

func TestSetMode_ErrosDeDominio_MapeiamStatus(t *testing.T) {
	casos := []struct {
		nome   string
		err    error
		status int
		code   string
	}{
		{"modeId inválido", domain.ErrInvalidModeID, http.StatusBadRequest, "invalid_mode_id"},
		{"cap de modos", domain.ErrTooManyModes, http.StatusBadRequest, "too_many_modes"},
		{"jogo inexistente", domain.ErrGameNotFound, http.StatusNotFound, "not_found"},
		{"params inválidos", domain.ErrInvalidProfileParams, http.StatusBadRequest, "invalid_profile_params"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			svc := new(mockService)
			svc.On("SetMode", mock.Anything, "g1", mock.Anything, mock.Anything, mock.Anything).
				Return(nil, c.err)
			rr := do(newServer(svc), http.MethodPut, "/games/g1/multiplayer-profiles/x",
				`{"archetype":"queue_arena","topology":"dedicated","params":{}}`)
			assert.Equal(t, c.status, rr.Code)
			assert.Contains(t, rr.Body.String(), c.code)
		})
	}
}

func TestListModes_200ComColecao(t *testing.T) {
	svc := new(mockService)
	svc.On("ListModes", mock.Anything, "g1").Return([]domain.MultiplayerMode{*sampleMode()}, nil)

	rr := do(newServer(svc), http.MethodGet, "/games/g1/multiplayer-profiles", "")
	require.Equal(t, http.StatusOK, rr.Code)

	var resp []map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.Len(t, resp, 1)
	assert.Equal(t, "tdm", resp[0]["modeId"])
}

func TestListModes_SemModos_200ListaVazia(t *testing.T) {
	svc := new(mockService)
	svc.On("ListModes", mock.Anything, "g1").Return(nil, nil)

	rr := do(newServer(svc), http.MethodGet, "/games/g1/multiplayer-profiles", "")
	require.Equal(t, http.StatusOK, rr.Code)
	assert.JSONEq(t, `[]`, rr.Body.String(), "lista vazia, não null")
}

func TestGetMode_Inexistente_404ModeNotFound(t *testing.T) {
	svc := new(mockService)
	svc.On("GetMode", mock.Anything, "g1", "nao").Return(nil, domain.ErrModeNotFound)

	rr := do(newServer(svc), http.MethodGet, "/games/g1/multiplayer-profiles/nao", "")
	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.Contains(t, rr.Body.String(), "mode_not_found")
}

func TestDeleteMode_Valido_204(t *testing.T) {
	svc := new(mockService)
	svc.On("DeleteMode", mock.Anything, "g1", "tdm").Return(nil)

	rr := do(newServer(svc), http.MethodDelete, "/games/g1/multiplayer-profiles/tdm", "")
	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestDeleteMode_Default_409Protegido(t *testing.T) {
	svc := new(mockService)
	svc.On("DeleteMode", mock.Anything, "g1", "default").Return(domain.ErrDefaultModeProtected)

	rr := do(newServer(svc), http.MethodDelete, "/games/g1/multiplayer-profiles/default", "")
	assert.Equal(t, http.StatusConflict, rr.Code)
	assert.Contains(t, rr.Body.String(), "default_mode_protected")
}

func TestGetGame_IncluiResumoDosModos(t *testing.T) {
	svc := new(mockService)
	m := sampleMode()
	g := &domain.Game{
		ID: "g1", Slug: "arena", Name: "Arena",
		MultiplayerProfile: &m.Profile,
		MultiplayerModes:   []domain.MultiplayerMode{*m},
	}
	svc.On("Get", mock.Anything, "g1").Return(g, nil)

	rr := do(newServer(svc), http.MethodGet, "/games/g1", "")
	require.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		MultiplayerProfile map[string]any   `json:"multiplayerProfile"`
		MultiplayerModes   []map[string]any `json:"multiplayerModes"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.NotNil(t, resp.MultiplayerProfile, "compat: perfil (default) segue no corpo")
	require.Len(t, resp.MultiplayerModes, 1)
	assert.Equal(t, "tdm", resp.MultiplayerModes[0]["modeId"])
	assert.Equal(t, "queue_arena", resp.MultiplayerModes[0]["archetype"])
	assert.Nil(t, resp.MultiplayerModes[0]["params"], "resumo não carrega params")
}

func TestGetGame_SemModos_ListaVazia(t *testing.T) {
	svc := new(mockService)
	svc.On("Get", mock.Anything, "g1").Return(&domain.Game{ID: "g1", Slug: "a", Name: "A"}, nil)

	rr := do(newServer(svc), http.MethodGet, "/games/g1", "")
	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	modes, ok := resp["multiplayerModes"].([]any)
	require.True(t, ok, "multiplayerModes presente como lista")
	assert.Empty(t, modes)
}
