package service

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/c130n/game-backend/services/identity-service/internal/domain"
)

type mockRepo struct{ mock.Mock }

func (m *mockRepo) Create(ctx context.Context, a *domain.Account) error {
	return m.Called(ctx, a).Error(0)
}

func (m *mockRepo) FindByHandle(ctx context.Context, handle string) (*domain.Account, error) {
	args := m.Called(ctx, handle)
	acc, _ := args.Get(0).(*domain.Account)
	return acc, args.Error(1)
}

func (m *mockRepo) ListAccounts(ctx context.Context, limit, offset int) ([]domain.Account, error) {
	args := m.Called(ctx, limit, offset)
	accs, _ := args.Get(0).([]domain.Account)
	return accs, args.Error(1)
}

func (m *mockRepo) CountAccounts(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
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

func TestRegister_HandleSenhaValidos_CriaContaSemVazarSenha(t *testing.T) {
	repo := new(mockRepo)
	var captured *domain.Account
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Account")).
		Return(nil).
		Run(func(args mock.Arguments) { captured = args.Get(1).(*domain.Account) })
	svc := New(repo, new(mockTokens))

	acc, err := svc.Register(context.Background(), "Vitor_1", "supersecret")

	require.NoError(t, err)
	assert.NotEmpty(t, acc.ID)
	assert.Equal(t, "Vitor_1", acc.Handle)
	assert.NotEqual(t, "supersecret", acc.PasswordHash) // nunca em claro
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(acc.PasswordHash), []byte("supersecret")))
	assert.Same(t, acc, captured)
	repo.AssertExpectations(t)
}

func TestRegister_HandleInvalido_RetornaErroSemPersistir(t *testing.T) {
	repo := new(mockRepo)
	svc := New(repo, new(mockTokens))

	_, err := svc.Register(context.Background(), "ab", "supersecret")

	assert.ErrorIs(t, err, domain.ErrInvalidHandle)
	repo.AssertNotCalled(t, "Create")
}

func TestRegister_SenhaFraca_RetornaErroSemPersistir(t *testing.T) {
	repo := new(mockRepo)
	svc := New(repo, new(mockTokens))

	_, err := svc.Register(context.Background(), "Vitor_1", "curta")

	assert.ErrorIs(t, err, domain.ErrWeakPassword)
	repo.AssertNotCalled(t, "Create")
}

func TestRegister_HandleDuplicado_RetornaErrHandleTaken(t *testing.T) {
	repo := new(mockRepo)
	repo.On("Create", mock.Anything, mock.Anything).Return(domain.ErrHandleTaken)
	svc := New(repo, new(mockTokens))

	_, err := svc.Register(context.Background(), "Vitor_1", "supersecret")

	assert.ErrorIs(t, err, domain.ErrHandleTaken)
}

func TestAuthenticate_CredenciaisCorretas_RetornaToken(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("supersecret"), bcrypt.DefaultCost)
	acc := &domain.Account{ID: "acc-1", Handle: "Vitor_1", PasswordHash: string(hash)}
	repo := new(mockRepo)
	repo.On("FindByHandle", mock.Anything, "Vitor_1").Return(acc, nil)
	tokens := new(mockTokens)
	tokens.On("Sign", mock.Anything, "acc-1", mock.Anything).Return("signed-token", nil)
	svc := New(repo, tokens)

	token, err := svc.Authenticate(context.Background(), "Vitor_1", "supersecret")

	require.NoError(t, err)
	assert.Equal(t, "signed-token", token)
	tokens.AssertExpectations(t)
}

func TestAuthenticate_SenhaIncorreta_RetornaInvalidCredentials(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("supersecret"), bcrypt.DefaultCost)
	acc := &domain.Account{ID: "acc-1", Handle: "Vitor_1", PasswordHash: string(hash)}
	repo := new(mockRepo)
	repo.On("FindByHandle", mock.Anything, "Vitor_1").Return(acc, nil)
	tokens := new(mockTokens)
	svc := New(repo, tokens)

	_, err := svc.Authenticate(context.Background(), "Vitor_1", "senhaErrada")

	assert.ErrorIs(t, err, domain.ErrInvalidCredentials)
	tokens.AssertNotCalled(t, "Sign")
}

func TestAuthenticate_HandleInexistente_RetornaInvalidCredentials(t *testing.T) {
	repo := new(mockRepo)
	repo.On("FindByHandle", mock.Anything, "fantasma").Return(nil, domain.ErrAccountNotFound)
	tokens := new(mockTokens)
	svc := New(repo, tokens)

	_, err := svc.Authenticate(context.Background(), "fantasma", "supersecret")

	// resposta idêntica ao caso de senha incorreta (não revela existência)
	assert.ErrorIs(t, err, domain.ErrInvalidCredentials)
	tokens.AssertNotCalled(t, "Sign")
}

func TestLoginAnonymous_EmiteTokenGuestAnonSemPersistir(t *testing.T) {
	tokens := new(mockTokens)
	var sub string
	var claims map[string]any
	tokens.On("Sign", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return("guest-token", nil).
		Run(func(args mock.Arguments) {
			sub = args.String(1)
			claims, _ = args.Get(2).(map[string]any)
		})
	repo := new(mockRepo)
	svc := New(repo, tokens)

	token, err := svc.LoginAnonymous(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "guest-token", token)
	assert.True(t, strings.HasPrefix(sub, "guest-"))
	assert.Equal(t, true, claims["anon"])
	repo.AssertNotCalled(t, "Create")
	repo.AssertNotCalled(t, "FindByHandle")
}

func TestLoginAnonymous_GuestsDistintos(t *testing.T) {
	tokens := new(mockTokens)
	subs := map[string]bool{}
	tokens.On("Sign", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return("t", nil).
		Run(func(args mock.Arguments) { subs[args.String(1)] = true })
	svc := New(new(mockRepo), tokens)

	_, _ = svc.LoginAnonymous(context.Background())
	_, _ = svc.LoginAnonymous(context.Background())

	assert.Len(t, subs, 2)
}
