package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/identity-service/internal/domain"
)

type mockAuth struct{ mock.Mock }

func (m *mockAuth) Register(ctx context.Context, handle, password string) (*domain.Account, error) {
	args := m.Called(ctx, handle, password)
	acc, _ := args.Get(0).(*domain.Account)
	return acc, args.Error(1)
}

func (m *mockAuth) Authenticate(ctx context.Context, handle, password, gameID string) (string, error) {
	args := m.Called(ctx, handle, password, gameID)
	return args.String(0), args.Error(1)
}

func (m *mockAuth) AuthenticateExternal(ctx context.Context, provider, ticket, gameID string) (string, error) {
	args := m.Called(ctx, provider, ticket, gameID)
	return args.String(0), args.Error(1)
}

func (m *mockAuth) LoginAnonymous(ctx context.Context, gameID string) (string, error) {
	args := m.Called(ctx, gameID)
	return args.String(0), args.Error(1)
}

type mockTokens struct{ mock.Mock }

func (m *mockTokens) Sign(ctx context.Context, subject string, claims map[string]any) (string, error) {
	args := m.Called(ctx, subject, claims)
	return args.String(0), args.Error(1)
}

func (m *mockTokens) JWKS(ctx context.Context) ([]byte, error) {
	args := m.Called(ctx)
	b, _ := args.Get(0).([]byte)
	return b, args.Error(1)
}

type mockLister struct{ mock.Mock }

func (m *mockLister) ListPlayers(ctx context.Context, limit, offset int) ([]domain.Account, int, error) {
	args := m.Called(ctx, limit, offset)
	accs, _ := args.Get(0).([]domain.Account)
	return accs, args.Int(1), args.Error(2)
}

func (m *mockLister) GetPlayer(ctx context.Context, id string) (*domain.Account, error) {
	args := m.Called(ctx, id)
	acc, _ := args.Get(0).(*domain.Account)
	return acc, args.Error(1)
}

func newServer(auth domain.AuthService, tokens domain.TokenIssuer) http.Handler {
	return newServerWith(auth, tokens, nil)
}

func newServerWith(auth domain.AuthService, tokens domain.TokenIssuer, players domain.PlayerLister) http.Handler {
	mux := http.NewServeMux()
	NewHandler(auth, tokens, players).Register(mux)
	return mux
}

func do(srv http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	return rr
}

func TestRegister_Valido_201SemSenha(t *testing.T) {
	auth := new(mockAuth)
	acc := &domain.Account{ID: "acc-1", Handle: "Vitor_1"}
	auth.On("Register", mock.Anything, "Vitor_1", "supersecret").Return(acc, nil)
	srv := newServer(auth, new(mockTokens))

	rr := do(srv, http.MethodPost, "/accounts", `{"handle":"Vitor_1","password":"supersecret"}`)

	require.Equal(t, http.StatusCreated, rr.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "acc-1", resp["id"])
	assert.Equal(t, "Vitor_1", resp["handle"])
	assert.NotContains(t, resp, "password")
	assert.NotContains(t, resp, "password_hash")
}

func TestRegister_HandleInvalido_400(t *testing.T) {
	auth := new(mockAuth)
	auth.On("Register", mock.Anything, mock.Anything, mock.Anything).Return(nil, domain.ErrInvalidHandle)
	srv := newServer(auth, new(mockTokens))

	rr := do(srv, http.MethodPost, "/accounts", `{"handle":"ab","password":"supersecret"}`)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestRegister_HandleDuplicado_409(t *testing.T) {
	auth := new(mockAuth)
	auth.On("Register", mock.Anything, mock.Anything, mock.Anything).Return(nil, domain.ErrHandleTaken)
	srv := newServer(auth, new(mockTokens))

	rr := do(srv, http.MethodPost, "/accounts", `{"handle":"Vitor_1","password":"supersecret"}`)
	assert.Equal(t, http.StatusConflict, rr.Code)
}

func TestLogin_Valido_200ComToken(t *testing.T) {
	auth := new(mockAuth)
	auth.On("Authenticate", mock.Anything, "Vitor_1", "supersecret", mock.Anything).Return("signed-token", nil)
	srv := newServer(auth, new(mockTokens))

	rr := do(srv, http.MethodPost, "/sessions", `{"handle":"Vitor_1","password":"supersecret"}`)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "signed-token", resp["token"])
}

func TestLogin_CredenciaisInvalidas_401(t *testing.T) {
	auth := new(mockAuth)
	auth.On("Authenticate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", domain.ErrInvalidCredentials)
	srv := newServer(auth, new(mockTokens))

	rr := do(srv, http.MethodPost, "/sessions", `{"handle":"x","password":"y"}`)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestJWKS_200(t *testing.T) {
	tokens := new(mockTokens)
	tokens.On("JWKS", mock.Anything).Return([]byte(`{"keys":[]}`), nil)
	srv := newServer(new(mockAuth), tokens)

	rr := do(srv, http.MethodGet, "/.well-known/jwks.json", "")

	require.Equal(t, http.StatusOK, rr.Code)
	assert.JSONEq(t, `{"keys":[]}`, rr.Body.String())
}

func TestLoginAnonymous_200ComToken(t *testing.T) {
	auth := new(mockAuth)
	auth.On("LoginAnonymous", mock.Anything, mock.Anything).Return("guest-token", nil)
	srv := newServer(auth, new(mockTokens))

	rr := do(srv, http.MethodPost, "/sessions/anonymous", "")

	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "guest-token", resp["token"])
}

func TestAdminListPlayers_RetornaListaETotal_SemHash(t *testing.T) {
	lister := new(mockLister)
	lister.On("ListPlayers", mock.Anything, 0, 0).Return([]domain.Account{
		{ID: "id-1", Handle: "Alice", PasswordHash: "SEGREDO", CreatedAt: time.Now().UTC()},
	}, 1, nil)
	srv := newServerWith(new(mockAuth), new(mockTokens), lister)

	rr := do(srv, http.MethodGet, "/admin/players", "")
	require.Equal(t, http.StatusOK, rr.Code)

	// o corpo jamais contém o hash nem o campo password
	assert.NotContains(t, rr.Body.String(), "SEGREDO")
	assert.NotContains(t, strings.ToLower(rr.Body.String()), "password")

	var resp struct {
		Total   int `json:"total"`
		Players []struct {
			ID        string `json:"id"`
			Handle    string `json:"handle"`
			CreatedAt string `json:"createdAt"`
		} `json:"players"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp.Total)
	require.Len(t, resp.Players, 1)
	assert.Equal(t, "Alice", resp.Players[0].Handle)
	assert.NotEmpty(t, resp.Players[0].CreatedAt)
}

func TestAdminListPlayers_RepassaPaginacaoDaQuery(t *testing.T) {
	lister := new(mockLister)
	lister.On("ListPlayers", mock.Anything, 5, 10).Return([]domain.Account{}, 0, nil)
	srv := newServerWith(new(mockAuth), new(mockTokens), lister)

	rr := do(srv, http.MethodGet, "/admin/players?limit=5&offset=10", "")
	require.Equal(t, http.StatusOK, rr.Code)
	lister.AssertExpectations(t) // confirma que 5/10 foram repassados ao service
}

func TestAdminGetPlayer_Existente_200SemHash(t *testing.T) {
	lister := new(mockLister)
	lister.On("GetPlayer", mock.Anything, "id-1").Return(
		&domain.Account{ID: "id-1", Handle: "Alice", PasswordHash: "SEGREDO", CreatedAt: time.Now().UTC()}, nil)
	srv := newServerWith(new(mockAuth), new(mockTokens), lister)

	rr := do(srv, http.MethodGet, "/admin/players/id-1", "")
	require.Equal(t, http.StatusOK, rr.Code)
	assert.NotContains(t, rr.Body.String(), "SEGREDO")

	var resp struct {
		ID     string `json:"id"`
		Handle string `json:"handle"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "id-1", resp.ID)
	assert.Equal(t, "Alice", resp.Handle)
}

func TestAdminGetPlayer_Inexistente_404(t *testing.T) {
	lister := new(mockLister)
	lister.On("GetPlayer", mock.Anything, "nope").Return(nil, domain.ErrAccountNotFound)
	srv := newServerWith(new(mockAuth), new(mockTokens), lister)

	rr := do(srv, http.MethodGet, "/admin/players/nope", "")
	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.Contains(t, rr.Body.String(), "player_not_found")
}

func TestAdminListPlayers_SemAuthorization_NaoBloqueia(t *testing.T) {
	lister := new(mockLister)
	lister.On("ListPlayers", mock.Anything, mock.Anything, mock.Anything).Return([]domain.Account{}, 0, nil)
	srv := newServerWith(new(mockAuth), new(mockTokens), lister)

	rr := do(srv, http.MethodGet, "/admin/players", "") // sem header Authorization
	assert.Equal(t, http.StatusOK, rr.Code)
}

// /accounts não exige auth: corpo acima do teto é recusado (400) sem chegar
// ao service — o decoder não bufferiza um corpo arbitrariamente grande.
func TestRegister_CorpoAcimaDoTeto_400SemTocarService(t *testing.T) {
	auth := new(mockAuth)
	srv := newServer(auth, new(mockTokens))

	body := `{"handle":"vitor","password":"` + strings.Repeat("x", maxBodyBytes) + `"}`
	rr := do(srv, http.MethodPost, "/accounts", body)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	auth.AssertNotCalled(t, "Register", mock.Anything, mock.Anything, mock.Anything)
}

// Spec 005-steam-login: o ticket vira o token de sempre; cada erro do
// provedor tem seu código HTTP.
func TestLoginExternal_Steam_200ComToken(t *testing.T) {
	auth := new(mockAuth)
	auth.On("AuthenticateExternal", mock.Anything, "steam", "abcd", "g1").Return("jwt", nil)
	rr := do(newServer(auth, new(mockTokens)), http.MethodPost, "/sessions/steam", `{"ticket":"abcd","gameId":"g1"}`)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.JSONEq(t, `{"token":"jwt"}`, rr.Body.String())
}

func TestLoginExternal_ErrosMapeiamStatus(t *testing.T) {
	cases := map[error]int{
		domain.ErrProviderNotSupported: http.StatusNotFound,
		domain.ErrInvalidTicket:        http.StatusUnauthorized,
		domain.ErrExternalBanned:       http.StatusForbidden,
		domain.ErrProviderUnavailable:  http.StatusServiceUnavailable,
	}
	for err, want := range cases {
		auth := new(mockAuth)
		auth.On("AuthenticateExternal", mock.Anything, "steam", "abcd", "").Return("", err)
		rr := do(newServer(auth, new(mockTokens)), http.MethodPost, "/sessions/steam", `{"ticket":"abcd"}`)
		assert.Equal(t, want, rr.Code, err.Error())
	}
}

func TestLoginExternal_AnonymousContinuaNaRotaLiteral(t *testing.T) {
	auth := new(mockAuth)
	auth.On("LoginAnonymous", mock.Anything, "").Return("guest-jwt", nil)
	rr := do(newServer(auth, new(mockTokens)), http.MethodPost, "/sessions/anonymous", ``)
	assert.Equal(t, http.StatusOK, rr.Code)
	auth.AssertNotCalled(t, "AuthenticateExternal", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}
