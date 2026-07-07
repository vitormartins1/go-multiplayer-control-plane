package registryclient

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-game-backend/services/matchmaking-service/internal/domain"
	"github.com/vitormartins1/go-game-backend/shared/auth"
)

// serverReturning cria um httptest.Server que devolve status+body para o
// endpoint do perfil e captura o header da internal key recebido.
func serverReturning(t *testing.T, status int, body string, gotKey *string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if gotKey != nil {
			*gotKey = r.Header.Get(auth.InternalKeyHeader)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestMatchSize_Ffa_UsaPlayerCount(t *testing.T) {
	srv := serverReturning(t, http.StatusOK,
		`{"archetype":"queue_arena","topology":"dedicated","params":{"structure":"ffa","playerCount":8}}`, nil)
	format, err := New(srv.URL, srv.Client()).Format(t.Context(), "g1")
	require.NoError(t, err)
	assert.Equal(t, 8, format.Size)
}

func TestMatchSize_Teams_MultiplicaContagemPorTamanho(t *testing.T) {
	srv := serverReturning(t, http.StatusOK,
		`{"archetype":"queue_arena","topology":"dedicated","params":{"structure":"teams","teamCount":2,"teamSize":3}}`, nil)
	format, err := New(srv.URL, srv.Client()).Format(t.Context(), "g1")
	require.NoError(t, err)
	assert.Equal(t, 6, format.Size, "teamCount*teamSize")
	assert.Equal(t, domain.StructureTeams, format.Structure)
	assert.Equal(t, 2, format.TeamCount)
	assert.Equal(t, 3, format.TeamSize)
}

func TestMatchSize_404_ErrProfileUnavailable(t *testing.T) {
	srv := serverReturning(t, http.StatusNotFound, `{"error":"profile_not_found"}`, nil)
	_, err := New(srv.URL, srv.Client()).Format(t.Context(), "g1")
	assert.ErrorIs(t, err, domain.ErrProfileUnavailable)
}

func TestMatchSize_ArchetypeNaoSuportado_ErrProfileUnavailable(t *testing.T) {
	// host_world_dropin não usa fila (usa invasão/sinais) — Format o rejeita.
	srv := serverReturning(t, http.StatusOK,
		`{"archetype":"host_world_dropin","topology":"dedicated","params":{"enabledRoles":["invader"],"invaderSlots":1}}`, nil)
	_, err := New(srv.URL, srv.Client()).Format(t.Context(), "g1")
	assert.ErrorIs(t, err, domain.ErrProfileUnavailable)
}

func TestMatchSize_BattleRoyaleSolo_FFA(t *testing.T) {
	srv := serverReturning(t, http.StatusOK,
		`{"archetype":"battle_royale","topology":"dedicated","params":{"lobbySize":100,"teamSize":1,"respawnMode":"one_life"}}`, nil)
	format, err := New(srv.URL, srv.Client()).Format(t.Context(), "g1")
	require.NoError(t, err)
	assert.Equal(t, domain.StructureFFA, format.Structure)
	assert.Equal(t, 100, format.Size)
}

func TestMatchSize_BattleRoyaleSquad_Squads(t *testing.T) {
	srv := serverReturning(t, http.StatusOK,
		`{"archetype":"battle_royale","topology":"dedicated","params":{"lobbySize":60,"teamSize":4,"respawnMode":"one_life"}}`, nil)
	format, err := New(srv.URL, srv.Client()).Format(t.Context(), "g1")
	require.NoError(t, err)
	assert.Equal(t, domain.StructureSquads, format.Structure)
	assert.Equal(t, 60, format.Size)
	assert.Equal(t, 4, format.TeamSize)
}

func TestMatchSize_BattleRoyale_LobbyZero_ErrProfileUnavailable(t *testing.T) {
	srv := serverReturning(t, http.StatusOK,
		`{"archetype":"battle_royale","topology":"dedicated","params":{"lobbySize":0,"teamSize":4}}`, nil)
	_, err := New(srv.URL, srv.Client()).Format(t.Context(), "g1")
	assert.ErrorIs(t, err, domain.ErrProfileUnavailable)
}

func TestMatchSize_BattleRoyale_TeamSizeZero_ErrProfileUnavailable(t *testing.T) {
	srv := serverReturning(t, http.StatusOK,
		`{"archetype":"battle_royale","topology":"dedicated","params":{"lobbySize":60,"teamSize":0}}`, nil)
	_, err := New(srv.URL, srv.Client()).Format(t.Context(), "g1")
	assert.ErrorIs(t, err, domain.ErrProfileUnavailable)
}

func TestMatchSize_CoopMission_TimeUnico(t *testing.T) {
	srv := serverReturning(t, http.StatusOK,
		`{"archetype":"coop_mission","topology":"dedicated","params":{"teamSize":4,"botfillEnabled":true}}`, nil)
	format, err := New(srv.URL, srv.Client()).Format(t.Context(), "g1")
	require.NoError(t, err)
	assert.Equal(t, domain.StructureTeams, format.Structure)
	assert.Equal(t, 1, format.TeamCount, "co-op = um único time")
	assert.Equal(t, 4, format.TeamSize)
	assert.Equal(t, 4, format.Size, "Size = teamSize (um time)")
}

func TestMatchSize_CoopMission_TeamSizeInvalido_ErrProfileUnavailable(t *testing.T) {
	srv := serverReturning(t, http.StatusOK,
		`{"archetype":"coop_mission","topology":"dedicated","params":{"teamSize":1}}`, nil)
	_, err := New(srv.URL, srv.Client()).Format(t.Context(), "g1")
	assert.ErrorIs(t, err, domain.ErrProfileUnavailable)
}

func TestMatchSize_TamanhoZero_ErrProfileUnavailable(t *testing.T) {
	srv := serverReturning(t, http.StatusOK,
		`{"archetype":"queue_arena","topology":"dedicated","params":{"structure":"ffa","playerCount":0}}`, nil)
	_, err := New(srv.URL, srv.Client()).Format(t.Context(), "g1")
	assert.ErrorIs(t, err, domain.ErrProfileUnavailable)
}

func TestInvasionPolicy_LeFlagsDoPerfil(t *testing.T) {
	srv := serverReturning(t, http.StatusOK,
		`{"archetype":"host_world_dropin","params":{"invasionEnabled":true,"levelRangeEnabled":true,"weaponTierEnabled":false}}`, nil)
	policy, err := New(srv.URL, srv.Client()).InvasionPolicy(t.Context(), "g1")
	require.NoError(t, err)
	assert.Equal(t, "host_world_dropin", policy.Archetype)
	assert.True(t, policy.InvasionEnabled)
	assert.True(t, policy.LevelRangeEnabled)
	assert.False(t, policy.WeaponTierEnabled)
	assert.True(t, policy.SupportsInvasion())
}

func TestInvasionPolicy_ArchetypeQueueArena_NaoSuporta(t *testing.T) {
	srv := serverReturning(t, http.StatusOK,
		`{"archetype":"queue_arena","params":{"structure":"ffa","playerCount":8}}`, nil)
	policy, err := New(srv.URL, srv.Client()).InvasionPolicy(t.Context(), "g1")
	require.NoError(t, err) // não é erro de leitura; o service decide ErrInvasionNotSupported
	assert.Equal(t, "queue_arena", policy.Archetype)
	assert.False(t, policy.SupportsInvasion())
}

func TestInvasionPolicy_404_ErrProfileUnavailable(t *testing.T) {
	srv := serverReturning(t, http.StatusNotFound, `{"error":"profile_not_found"}`, nil)
	_, err := New(srv.URL, srv.Client()).InvasionPolicy(t.Context(), "g1")
	assert.ErrorIs(t, err, domain.ErrProfileUnavailable)
}

func TestCoopPolicy_EnabledRolesComCoop_SuportaCoop(t *testing.T) {
	srv := serverReturning(t, http.StatusOK,
		`{"archetype":"host_world_dropin","params":{"enabledRoles":["coop","invader"],"levelRangeEnabled":true,"weaponTierEnabled":true}}`, nil)
	policy, err := New(srv.URL, srv.Client()).CoopPolicy(t.Context(), "g1")
	require.NoError(t, err)
	assert.True(t, policy.CoopEnabled)
	assert.True(t, policy.LevelRangeEnabled)
	assert.True(t, policy.WeaponTierEnabled)
	assert.True(t, policy.SupportsCoop())
}

func TestCoopPolicy_SemCoopNosRoles_NaoSuporta(t *testing.T) {
	srv := serverReturning(t, http.StatusOK,
		`{"archetype":"host_world_dropin","params":{"enabledRoles":["invader"]}}`, nil)
	policy, err := New(srv.URL, srv.Client()).CoopPolicy(t.Context(), "g1")
	require.NoError(t, err)
	assert.False(t, policy.CoopEnabled)
	assert.False(t, policy.SupportsCoop(), "só invasão habilitada")
}

func TestCoopPolicy_404_ErrProfileUnavailable(t *testing.T) {
	srv := serverReturning(t, http.StatusNotFound, `{"error":"profile_not_found"}`, nil)
	_, err := New(srv.URL, srv.Client()).CoopPolicy(t.Context(), "g1")
	assert.ErrorIs(t, err, domain.ErrProfileUnavailable)
}

func TestMatchSize_InjetaInternalKey(t *testing.T) {
	var gotKey string
	srv := serverReturning(t, http.StatusOK,
		`{"archetype":"queue_arena","topology":"dedicated","params":{"structure":"ffa","playerCount":2}}`, &gotKey)

	httpClient := &http.Client{Transport: auth.InternalKeyTransport("segredo", nil)}
	_, err := New(srv.URL, httpClient).Format(t.Context(), "g1")
	require.NoError(t, err)
	assert.Equal(t, "segredo", gotKey, "a internal key viaja no header")
}
