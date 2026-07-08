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

	"github.com/vitormartins1/go-multiplayer-control-plane/services/identity-service/internal/domain"
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

// seedAccounts cria n contas com created_at crescente (older -> newer).
func seedAccounts(t *testing.T, repo *PostgresAccountRepository, handles ...string) {
	t.Helper()
	ctx := context.Background()
	base := time.Now().UTC().Add(-time.Hour)
	for i, h := range handles {
		a := newAccount(h)
		a.CreatedAt = base.Add(time.Duration(i) * time.Minute)
		require.NoError(t, repo.Create(ctx, a))
	}
}

func TestPostgresAccountRepository_ListAccounts_OrdemDescESemHash(t *testing.T) {
	repo := NewPostgresAccountRepository(newTestPool(t))
	seedAccounts(t, repo, "Alice", "Bob", "Carol") // Carol é a mais recente

	got, err := repo.ListAccounts(context.Background(), 50, 0)
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, []string{"Carol", "Bob", "Alice"}, []string{got[0].Handle, got[1].Handle, got[2].Handle})
	for _, a := range got {
		assert.Empty(t, a.PasswordHash, "password_hash nunca é lido no caminho admin")
	}
}

func TestPostgresAccountRepository_ListAccounts_Paginacao(t *testing.T) {
	repo := NewPostgresAccountRepository(newTestPool(t))
	seedAccounts(t, repo, "Alice", "Bob", "Carol", "Dave")

	page1, err := repo.ListAccounts(context.Background(), 2, 0)
	require.NoError(t, err)
	require.Len(t, page1, 2)
	assert.Equal(t, []string{"Dave", "Carol"}, []string{page1[0].Handle, page1[1].Handle})

	page2, err := repo.ListAccounts(context.Background(), 2, 2)
	require.NoError(t, err)
	require.Len(t, page2, 2)
	assert.Equal(t, []string{"Bob", "Alice"}, []string{page2[0].Handle, page2[1].Handle})
}

func TestPostgresAccountRepository_FindByID_ProjecaoSemHash(t *testing.T) {
	repo := NewPostgresAccountRepository(newTestPool(t))
	ctx := context.Background()

	acc := newAccount("Vitor")
	require.NoError(t, repo.Create(ctx, acc))

	got, err := repo.FindByID(ctx, acc.ID)
	require.NoError(t, err)
	assert.Equal(t, "Vitor", got.Handle)
	assert.Empty(t, got.PasswordHash, "password_hash nunca é lido no caminho admin")
}

func TestPostgresAccountRepository_FindByID_Inexistente_RetornaErrAccountNotFound(t *testing.T) {
	repo := NewPostgresAccountRepository(newTestPool(t))
	_, err := repo.FindByID(context.Background(), uuid.NewString())
	assert.ErrorIs(t, err, domain.ErrAccountNotFound)
}

func TestPostgresAccountRepository_FindByID_UUIDInvalido_RetornaErrAccountNotFound(t *testing.T) {
	repo := NewPostgresAccountRepository(newTestPool(t))
	_, err := repo.FindByID(context.Background(), "nao-e-uuid")
	assert.ErrorIs(t, err, domain.ErrAccountNotFound)
}

func TestPostgresAccountRepository_CountAccounts(t *testing.T) {
	repo := NewPostgresAccountRepository(newTestPool(t))
	ctx := context.Background()

	n0, err := repo.CountAccounts(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, n0)

	seedAccounts(t, repo, "Alice", "Bob")
	n2, err := repo.CountAccounts(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, n2)
}
