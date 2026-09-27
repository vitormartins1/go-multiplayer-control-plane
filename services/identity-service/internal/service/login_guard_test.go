package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/identity-service/internal/domain"
)

// spec 007 — limite de falhas do login por senha.

func guardedSvc(t *testing.T, now *time.Time) (domain.AuthService, *mockRepo, *mockTokens) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("supersecret"), bcrypt.MinCost)
	require.NoError(t, err)
	repo := new(mockRepo)
	repo.On("FindByHandle", mock.Anything, "Vitor_1").Return(&domain.Account{ID: "acc-1", Handle: "Vitor_1", PasswordHash: string(hash)}, nil)
	repo.On("FindByHandle", mock.Anything, mock.Anything).Return(nil, domain.ErrAccountNotFound)
	tokens := new(mockTokens)
	tokens.On("Sign", mock.Anything, "acc-1", mock.Anything).Return("tok", nil)
	return New(repo, tokens, nil, nil, WithLoginLimit(3, 15*time.Minute, func() time.Time { return *now })), repo, tokens
}

func TestLoginGuard_BloqueiaDepoisDoLimiteELiberaAposJanela(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	svc, repo, _ := guardedSvc(t, &now)
	ctx := context.Background()

	for range 3 {
		_, err := svc.Authenticate(ctx, "Vitor_1", "errada", "")
		require.ErrorIs(t, err, domain.ErrInvalidCredentials)
	}
	_, err := svc.Authenticate(ctx, "vitor_1", "supersecret", "") // maiúsculas: mesmo handle
	assert.ErrorIs(t, err, domain.ErrTooManyAttempts, "bloqueado mesmo com a senha certa")
	repo.AssertNumberOfCalls(t, "FindByHandle", 3) // bloqueado nem consulta o banco

	now = now.Add(15 * time.Minute)
	tok, err := svc.Authenticate(ctx, "Vitor_1", "supersecret", "")
	require.NoError(t, err, "janela venceu")
	assert.Equal(t, "tok", tok)
}

func TestLoginGuard_SucessoZeraOContador(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	svc, _, _ := guardedSvc(t, &now)
	ctx := context.Background()

	for range 2 {
		_, err := svc.Authenticate(ctx, "Vitor_1", "errada", "")
		require.ErrorIs(t, err, domain.ErrInvalidCredentials)
	}
	_, err := svc.Authenticate(ctx, "Vitor_1", "supersecret", "")
	require.NoError(t, err)
	for range 2 {
		_, err = svc.Authenticate(ctx, "Vitor_1", "errada", "")
		require.ErrorIs(t, err, domain.ErrInvalidCredentials, "contador recomeçou do zero")
	}
}

func TestLoginGuard_HandleInexistenteTambemBloqueia(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	svc, _, _ := guardedSvc(t, &now)
	ctx := context.Background()
	for range 3 {
		_, err := svc.Authenticate(ctx, "fantasma", "x", "")
		require.ErrorIs(t, err, domain.ErrInvalidCredentials)
	}
	_, err := svc.Authenticate(ctx, "fantasma", "x", "")
	assert.ErrorIs(t, err, domain.ErrTooManyAttempts, "mesma resposta que um handle real: não revela existência")
}

func TestLoginGuard_PruneDescartaJanelasVencidas(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	g := newLoginGuard(3, time.Minute, func() time.Time { return now })
	g.fail("a")
	g.fail("b")
	now = now.Add(2 * time.Minute)
	g.fail("c")
	g.mu.Lock()
	g.pruneLocked(now)
	n := len(g.failures)
	g.mu.Unlock()
	assert.Equal(t, 1, n, "só a janela de c continua valendo")
	assert.False(t, g.blocked("a"))
}
