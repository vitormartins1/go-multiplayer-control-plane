//go:build integration

// Testes de integração com Postgres real (testcontainers). Rodar com:
//
//	go test -tags=integration ./services/identity-service/internal/repository/
//
// Requer Docker em execução.
package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/c130n/game-backend/services/identity-service/internal/domain"
)

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	ctr, err := tcpostgres.Run(ctx, "postgres:16",
		tcpostgres.WithDatabase("identity"),
		tcpostgres.WithUsername("dev"),
		tcpostgres.WithPassword("dev"),
		tcpostgres.WithInitScripts("../../migrations/001_create_accounts.sql"),
		tcpostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func newAccount(handle string) *domain.Account {
	return &domain.Account{
		ID:           uuid.NewString(),
		Handle:       handle,
		PasswordHash: "hash",
		CreatedAt:    time.Now().UTC(),
	}
}

func TestPostgresAccountRepository_CreateEFindCaseInsensitive(t *testing.T) {
	repo := NewPostgresAccountRepository(newTestPool(t))
	ctx := context.Background()

	acc := newAccount("Vitor_1")
	require.NoError(t, repo.Create(ctx, acc))

	got, err := repo.FindByHandle(ctx, "vitor_1") // busca em case diferente
	require.NoError(t, err)
	assert.Equal(t, acc.ID, got.ID)
	assert.Equal(t, "Vitor_1", got.Handle)
}

func TestPostgresAccountRepository_HandleDuplicadoCaseInsensitive_RetornaErrHandleTaken(t *testing.T) {
	repo := NewPostgresAccountRepository(newTestPool(t))
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, newAccount("Vitor")))
	err := repo.Create(ctx, newAccount("vitor")) // mesmo handle, case diferente
	assert.ErrorIs(t, err, domain.ErrHandleTaken)
}

func TestPostgresAccountRepository_FindInexistente_RetornaErrAccountNotFound(t *testing.T) {
	repo := NewPostgresAccountRepository(newTestPool(t))
	_, err := repo.FindByHandle(context.Background(), "fantasma")
	assert.ErrorIs(t, err, domain.ErrAccountNotFound)
}
