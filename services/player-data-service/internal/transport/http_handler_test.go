package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/player-data-service/internal/domain"
)

type fakeVerifier struct {
	playerID string
	err      error
}

func (f fakeVerifier) Verify(_ context.Context, _ string) (string, error) {
	return f.playerID, f.err
}

type fakeService struct {
	getValue    []byte
	getErr      error
	nsValue     map[string][]byte
	putErr      error
	putBody     []byte
	listValue   []domain.Entry
	gotGameID   string
	gotPlayerID string
}

func (f *fakeService) Get(_ context.Context, gameID, playerID, _, _ string) ([]byte, error) {
	f.gotGameID = gameID
	f.gotPlayerID = playerID
	return f.getValue, f.getErr
}

func (f *fakeService) GetNamespace(_ context.Context, gameID, playerID, _ string) (map[string][]byte, error) {
	f.gotGameID = gameID
	f.gotPlayerID = playerID
	return f.nsValue, nil
}

func (f *fakeService) Put(_ context.Context, gameID, playerID, _, _ string, value []byte) error {
	f.gotGameID = gameID
	f.gotPlayerID = playerID
	f.putBody = value
	return f.putErr
}

func (f *fakeService) Delete(_ context.Context, gameID, playerID, _, _ string) error {
	f.gotGameID = gameID
	f.gotPlayerID = playerID
	return nil
}

func (f *fakeService) ListForPlayer(_ context.Context, gameID, playerID string) ([]domain.Entry, error) {
	f.gotGameID = gameID
	f.gotPlayerID = playerID
	return f.listValue, nil
}

func newServer(svc domain.PlayerDataService, v domain.TokenVerifier) *httptest.Server {
	mux := http.NewServeMux()
	NewHandler(svc, v).Register(mux)
	return httptest.NewServer(mux)
}

// do dispara uma requisição player-facing (Bearer + X-Game-Id). game vazio
// omite o header X-Game-Id (caso missing_game).
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

func TestPut_Grava_204(t *testing.T) {
	svc := &fakeService{}
	srv := newServer(svc, fakeVerifier{playerID: "p1"})
	defer srv.Close()

	resp := do(t, srv, http.MethodPut, "/me/data/inventory/gold", "tok", "g1", `{"amount":100}`)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.JSONEq(t, `{"amount":100}`, string(svc.putBody))
	assert.Equal(t, "g1", svc.gotGameID)
	assert.Equal(t, "p1", svc.gotPlayerID)
}

func TestPut_SemGame_400(t *testing.T) {
	svc := &fakeService{}
	srv := newServer(svc, fakeVerifier{playerID: "p1"})
	defer srv.Close()

	resp := do(t, srv, http.MethodPut, "/me/data/inventory/gold", "tok", "", `{"amount":100}`)
	defer resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Empty(t, svc.gotGameID) // não chegou ao service
}

func TestGet_SemGame_400(t *testing.T) {
	srv := newServer(&fakeService{}, fakeVerifier{playerID: "p1"})
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/me/data/inventory/gold", "tok", "", "")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestPut_CorpoNaoJSON_400(t *testing.T) {
	srv := newServer(&fakeService{}, fakeVerifier{playerID: "p1"})
	defer srv.Close()

	resp := do(t, srv, http.MethodPut, "/me/data/inventory/gold", "tok", "g1", `{nao-eh-json`)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestPut_KeyInvalida_400(t *testing.T) {
	svc := &fakeService{putErr: domain.ErrInvalidKey}
	srv := newServer(svc, fakeVerifier{playerID: "p1"})
	defer srv.Close()

	resp := do(t, srv, http.MethodPut, "/me/data/inventory/gold", "tok", "g1", `1`)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGet_RetornaValor_200(t *testing.T) {
	svc := &fakeService{getValue: []byte(`{"amount":100}`)}
	srv := newServer(svc, fakeVerifier{playerID: "p9"})
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/me/data/inventory/gold", "tok", "g1", "")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	b, _ := io.ReadAll(resp.Body)
	assert.JSONEq(t, `{"amount":100}`, string(b))
	assert.Equal(t, "g1", svc.gotGameID)
	assert.Equal(t, "p9", svc.gotPlayerID)
}

func TestGet_Inexistente_404(t *testing.T) {
	svc := &fakeService{getErr: domain.ErrNotFound}
	srv := newServer(svc, fakeVerifier{playerID: "p1"})
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/me/data/inventory/fantasma", "tok", "g1", "")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestGetNamespace_RetornaObjeto_200(t *testing.T) {
	svc := &fakeService{nsValue: map[string][]byte{"kills": []byte(`10`), "deaths": []byte(`2`)}}
	srv := newServer(svc, fakeVerifier{playerID: "p1"})
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/me/data/stats", "tok", "g1", "")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var got map[string]json.RawMessage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	assert.JSONEq(t, `10`, string(got["kills"]))
	assert.JSONEq(t, `2`, string(got["deaths"]))
}

func TestDelete_204(t *testing.T) {
	srv := newServer(&fakeService{}, fakeVerifier{playerID: "p1"})
	defer srv.Close()

	resp := do(t, srv, http.MethodDelete, "/me/data/inventory/gold", "tok", "g1", "")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestAuth_SemBearer_401(t *testing.T) {
	srv := newServer(&fakeService{}, fakeVerifier{playerID: "p1"})
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/me/data/inventory/gold", "", "g1", "")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAuth_TokenInvalido_401(t *testing.T) {
	srv := newServer(&fakeService{}, fakeVerifier{err: errors.New("token inválido")})
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/me/data/inventory/gold", "qualquer", "g1", "")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAdminListForPlayer_RetornaLista_200(t *testing.T) {
	svc := &fakeService{listValue: []domain.Entry{
		{Namespace: "inventory", Key: "gold", Value: []byte(`100`)},
		{Namespace: "stats", Key: "kills", Value: []byte(`{"total":10}`)},
	}}
	srv := newServer(svc, fakeVerifier{}) // admin não usa o verifier
	defer srv.Close()

	// admin não exige Bearer nem X-Game-Id (jogo/jogador no path).
	resp := do(t, srv, http.MethodGet, "/admin/games/g1/players/p1/data", "", "", "")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "g1", svc.gotGameID)
	assert.Equal(t, "p1", svc.gotPlayerID)

	var got struct {
		Data []struct {
			Namespace string          `json:"namespace"`
			Key       string          `json:"key"`
			Value     json.RawMessage `json:"value"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	require.Len(t, got.Data, 2)
	assert.Equal(t, "inventory", got.Data[0].Namespace)
	assert.JSONEq(t, `100`, string(got.Data[0].Value))
	assert.JSONEq(t, `{"total":10}`, string(got.Data[1].Value))
}

// Spec 003: valor acima de 400 KiB -> 413 sem chegar ao service; no limite -> 204.
func TestPut_ValorAcimaDoLimite_413(t *testing.T) {
	svc := &fakeService{}
	srv := newServer(svc, fakeVerifier{playerID: "p1"})
	defer srv.Close()

	big := `"` + strings.Repeat("x", maxValueBytes-1) + `"` // JSON válido com maxValueBytes+1 bytes
	resp := do(t, srv, http.MethodPut, "/me/data/inventory/gold", "tok", "g1", big)
	defer resp.Body.Close()
	require.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
	assert.Empty(t, svc.gotGameID, "não chegou ao service")
}

func TestPut_ValorNoLimite_204(t *testing.T) {
	svc := &fakeService{}
	srv := newServer(svc, fakeVerifier{playerID: "p1"})
	defer srv.Close()

	exact := `"` + strings.Repeat("x", maxValueBytes-2) + `"` // exatamente maxValueBytes
	resp := do(t, srv, http.MethodPut, "/me/data/inventory/gold", "tok", "g1", exact)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Len(t, svc.putBody, maxValueBytes)
}
