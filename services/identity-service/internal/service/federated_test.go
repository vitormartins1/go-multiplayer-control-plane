package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/identity-service/internal/domain"
)

type mockExternalRepo struct{ mock.Mock }

func (m *mockExternalRepo) FindByExternal(ctx context.Context, provider, externalID string) (*domain.Account, error) {
	args := m.Called(ctx, provider, externalID)
	acc, _ := args.Get(0).(*domain.Account)
	return acc, args.Error(1)
}

func (m *mockExternalRepo) CreateWithExternal(ctx context.Context, a *domain.Account, provider, externalID string) error {
	return m.Called(ctx, a, provider, externalID).Error(0)
}

type mockVerifier struct{ mock.Mock }

func (m *mockVerifier) Provider() string { return "steam" }

func (m *mockVerifier) Verify(ctx context.Context, ticket string) (domain.ExternalIdentity, error) {
	args := m.Called(ctx, ticket)
	return args.Get(0).(domain.ExternalIdentity), args.Error(1)
}

const steamID = "76561198000000042"

func steamIdentity() domain.ExternalIdentity {
	return domain.ExternalIdentity{Provider: "steam", ExternalID: steamID}
}

func federatedSetup(t *testing.T) (domain.AuthService, *mockExternalRepo, *mockVerifier, *mockTokens) {
	t.Helper()
	ext, ver, tok := &mockExternalRepo{}, &mockVerifier{}, &mockTokens{}
	svc := New(&mockRepo{}, tok, nil, nil, WithExternalLogin(ext, ver))
	return svc, ext, ver, tok
}

func TestAuthenticateExternal_PrimeiraVez_CriaContaSemSenhaEVincula(t *testing.T) {
	svc, ext, ver, tok := federatedSetup(t)
	ver.On("Verify", mock.Anything, "abcd").Return(steamIdentity(), nil)
	ext.On("FindByExternal", mock.Anything, "steam", steamID).Return(nil, domain.ErrAccountNotFound)
	var created *domain.Account
	ext.On("CreateWithExternal", mock.Anything, mock.Anything, "steam", steamID).
		Run(func(args mock.Arguments) { created = args.Get(1).(*domain.Account) }).Return(nil)
	tok.On("Sign", mock.Anything, mock.Anything, map[string]any{"idp": "steam"}).Return("jwt", nil)

	got, err := svc.AuthenticateExternal(context.Background(), "steam", "abcd", "")
	require.NoError(t, err)
	assert.Equal(t, "jwt", got)
	require.NotNil(t, created)
	assert.Equal(t, "steam_61198000000042", created.Handle, "steam_ + 14 últimos dígitos (20 chars)")
	assert.Empty(t, created.PasswordHash, "conta federada não tem senha")
	tok.AssertCalled(t, "Sign", mock.Anything, created.ID, map[string]any{"idp": "steam"})
}

func TestAuthenticateExternal_JaVinculado_ReusaConta(t *testing.T) {
	svc, ext, ver, tok := federatedSetup(t)
	ver.On("Verify", mock.Anything, "abcd").Return(steamIdentity(), nil)
	ext.On("FindByExternal", mock.Anything, "steam", steamID).Return(&domain.Account{ID: "acc-1", Handle: "steam_x"}, nil)
	tok.On("Sign", mock.Anything, "acc-1", map[string]any{"idp": "steam"}).Return("jwt", nil)

	_, err := svc.AuthenticateExternal(context.Background(), "steam", "abcd", "")
	require.NoError(t, err)
	ext.AssertNotCalled(t, "CreateWithExternal", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestAuthenticateExternal_CorridaNoVinculo_ReleAConta(t *testing.T) {
	svc, ext, ver, tok := federatedSetup(t)
	ver.On("Verify", mock.Anything, "abcd").Return(steamIdentity(), nil)
	ext.On("FindByExternal", mock.Anything, "steam", steamID).Return(nil, domain.ErrAccountNotFound).Once()
	ext.On("CreateWithExternal", mock.Anything, mock.Anything, "steam", steamID).Return(domain.ErrExternalLinked)
	ext.On("FindByExternal", mock.Anything, "steam", steamID).Return(&domain.Account{ID: "acc-outro-login"}, nil).Once()
	tok.On("Sign", mock.Anything, "acc-outro-login", mock.Anything).Return("jwt", nil)

	_, err := svc.AuthenticateExternal(context.Background(), "steam", "abcd", "")
	require.NoError(t, err)
}

func TestAuthenticateExternal_HandleOcupado_TentaAlternativo(t *testing.T) {
	svc, ext, ver, tok := federatedSetup(t)
	ver.On("Verify", mock.Anything, "abcd").Return(steamIdentity(), nil)
	ext.On("FindByExternal", mock.Anything, "steam", steamID).Return(nil, domain.ErrAccountNotFound)
	var handles []string
	ext.On("CreateWithExternal", mock.Anything, mock.Anything, "steam", steamID).
		Run(func(args mock.Arguments) { handles = append(handles, args.Get(1).(*domain.Account).Handle) }).
		Return(domain.ErrHandleTaken).Once()
	ext.On("CreateWithExternal", mock.Anything, mock.Anything, "steam", steamID).
		Run(func(args mock.Arguments) { handles = append(handles, args.Get(1).(*domain.Account).Handle) }).
		Return(nil).Once()
	tok.On("Sign", mock.Anything, mock.Anything, mock.Anything).Return("jwt", nil)

	_, err := svc.AuthenticateExternal(context.Background(), "steam", "abcd", "")
	require.NoError(t, err)
	require.Len(t, handles, 2)
	assert.NotEqual(t, handles[0], handles[1])
	assert.Regexp(t, `^steam_[0-9a-f]{8}$`, handles[1])
}

func TestAuthenticateExternal_ErrosDoProvedorPropagam(t *testing.T) {
	for _, want := range []error{domain.ErrInvalidTicket, domain.ErrExternalBanned, domain.ErrProviderUnavailable} {
		svc, _, ver, _ := federatedSetup(t)
		ver.On("Verify", mock.Anything, "abcd").Return(domain.ExternalIdentity{}, want)
		_, err := svc.AuthenticateExternal(context.Background(), "steam", "abcd", "")
		assert.ErrorIs(t, err, want)
	}
}

func TestAuthenticateExternal_ProvedorDesconhecidoOuSemConfig_ErrProviderNotSupported(t *testing.T) {
	svc, _, _, _ := federatedSetup(t)
	_, err := svc.AuthenticateExternal(context.Background(), "gog", "abcd", "")
	assert.ErrorIs(t, err, domain.ErrProviderNotSupported)

	plain := New(&mockRepo{}, &mockTokens{}, nil, nil) // sem WithExternalLogin
	_, err = plain.AuthenticateExternal(context.Background(), "steam", "abcd", "")
	assert.ErrorIs(t, err, domain.ErrProviderNotSupported)
}

// Revisão: os dois handles colidindo é falha nossa — não sai como handle_taken.
func TestAuthenticateExternal_DoisHandlesOcupados_NaoViraHandleTaken(t *testing.T) {
	svc, ext, ver, _ := federatedSetup(t)
	ver.On("Verify", mock.Anything, "abcd").Return(steamIdentity(), nil)
	ext.On("FindByExternal", mock.Anything, "steam", steamID).Return(nil, domain.ErrAccountNotFound)
	ext.On("CreateWithExternal", mock.Anything, mock.Anything, "steam", steamID).Return(domain.ErrHandleTaken)

	_, err := svc.AuthenticateExternal(context.Background(), "steam", "abcd", "")
	require.Error(t, err)
	assert.NotErrorIs(t, err, domain.ErrHandleTaken)
}
