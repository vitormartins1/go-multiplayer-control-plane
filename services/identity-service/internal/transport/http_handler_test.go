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

	"github.com/c130n/game-backend/services/identity-service/internal/domain"
)

type mockAuth struct{ mock.Mock }

func (m *mockAuth) Register(ctx context.Context, handle, password string) (*domain.Account, error) {
	args := m.Called(ctx, handle, password)
	acc, _ := args.Get(0).(*domain.Account)
	return acc, args.Error(1)
}

func (m *mockAuth) Authenticate(ctx context.Context, handle, password string) (string, error) {
	args := m.Called(ctx, handle, password)
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

func newServer(auth domain.AuthService, tokens domain.TokenIssuer) http.Handler {
	mux := http.NewServeMux()
	NewHandler(auth, tokens).Register(mux)
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
	auth.On("Authenticate", mock.Anything, "Vitor_1", "supersecret").Return("signed-token", nil)
	srv := newServer(auth, new(mockTokens))

	rr := do(srv, http.MethodPost, "/sessions", `{"handle":"Vitor_1","password":"supersecret"}`)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "signed-token", resp["token"])
}

func TestLogin_CredenciaisInvalidas_401(t *testing.T) {
	auth := new(mockAuth)
	auth.On("Authenticate", mock.Anything, mock.Anything, mock.Anything).Return("", domain.ErrInvalidCredentials)
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
