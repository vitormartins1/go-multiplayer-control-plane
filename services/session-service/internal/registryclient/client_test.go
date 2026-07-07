package registryclient

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/domain"
	"github.com/vitormartins1/go-multiplayer-control-plane/shared/auth"
)

// serverReturning cria um httptest.Server que devolve status+body para o
// endpoint do perfil e captura a internal key recebida.
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

func TestModeProfile_HostWorld_LePapeisECapacidade(t *testing.T) {
	srv := serverReturning(t, http.StatusOK,
		`{"archetype":"host_world_dropin","topology":"dedicated","params":{"enabledRoles":["coop","invader"],"coopSlots":3,"invaderSlots":2}}`, nil)
	p, err := New(srv.URL, srv.Client()).ModeProfile(t.Context(), "g1", "default")
	require.NoError(t, err)
	assert.Equal(t, "host_world_dropin", p.Archetype)
	assert.Equal(t, []string{"coop", "invader"}, p.EnabledRoles)
	assert.Equal(t, 3, p.CoopSlots)
	assert.Equal(t, 2, p.InvaderSlots)
}

func TestModeProfile_404_ErrProfileUnavailable(t *testing.T) {
	srv := serverReturning(t, http.StatusNotFound, `{"error":"profile_not_found"}`, nil)
	_, err := New(srv.URL, srv.Client()).ModeProfile(t.Context(), "g1", "default")
	assert.ErrorIs(t, err, domain.ErrProfileUnavailable)
}

// O client é genérico por modo (ADR-0024): qualquer arquétipo vira snapshot com
// params crus + hash — a checagem host_world subiu para o service.
func TestModeProfile_QueueArena_SnapshotComParamsCrusEHash(t *testing.T) {
	srv := serverReturning(t, http.StatusOK,
		`{"archetype":"queue_arena","topology":"dedicated","params":{"structure":"ffa","playerCount":8}}`, nil)
	p, err := New(srv.URL, srv.Client()).ModeProfile(t.Context(), "g1", "tdm")
	require.NoError(t, err)
	assert.Equal(t, "queue_arena", p.Archetype)
	assert.Equal(t, "tdm", p.ModeID)
	assert.JSONEq(t, `{"structure":"ffa","playerCount":8}`, string(p.ParamsJSON), "params crus preservados p/ o manifest")
	assert.Len(t, p.Hash, 16, "hash curto determinístico")
	assert.Empty(t, p.EnabledRoles, "derivados de host_world ficam zero")
}

func TestModeProfile_InjetaInternalKey(t *testing.T) {
	var gotKey string
	srv := serverReturning(t, http.StatusOK,
		`{"archetype":"host_world_dropin","topology":"dedicated","params":{"enabledRoles":["coop"],"coopSlots":1}}`, &gotKey)

	httpClient := &http.Client{Transport: auth.InternalKeyTransport("segredo", nil)}
	_, err := New(srv.URL, httpClient).ModeProfile(t.Context(), "g1", "default")
	require.NoError(t, err)
	assert.Equal(t, "segredo", gotKey, "a internal key viaja no header")
}
