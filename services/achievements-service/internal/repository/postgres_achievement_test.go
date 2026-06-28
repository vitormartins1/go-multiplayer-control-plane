//go:build integration

// Testes de integração com Postgres real (testcontainers). Rodar com:
//
//	go test -tags=integration ./services/achievements-service/internal/repository/
//
// Requer Docker em execução.
package repository

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	ctr, err := tcpostgres.Run(ctx, "postgres:16",
		tcpostgres.WithDatabase("achievements"),
		tcpostgres.WithUsername("dev"),
		tcpostgres.WithPassword("dev"),
		tcpostgres.WithInitScripts("../../migrations/001_create_achievements.sql"),
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

func TestListCatalog_CarregaSeed(t *testing.T) {
	repo := NewPostgresAchievementRepository(newTestPool(t))
	cat, err := repo.ListCatalog(context.Background())
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(cat), 3)

	ids := map[string]bool{}
	for _, a := range cat {
		ids[a.ID] = true
	}
	assert.True(t, ids["first_win"])
}

func TestFindByEventType_RetornaConquistasDoTipo(t *testing.T) {
	repo := NewPostgresAchievementRepository(newTestPool(t))
	got, err := repo.FindByEventType(context.Background(), "match_won")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "first_win", got[0].ID)
}

func TestUnlock_Idempotente_NaoAlteraUnlockedAt(t *testing.T) {
	repo := NewPostgresAchievementRepository(newTestPool(t))
	ctx := context.Background()

	at1 := time.Now().UTC()
	created, err := repo.Unlock(ctx, "player-1", "first_win", at1)
	require.NoError(t, err)
	assert.True(t, created, "primeiro unlock deve criar")

	at2 := at1.Add(time.Hour)
	created, err = repo.Unlock(ctx, "player-1", "first_win", at2)
	require.NoError(t, err)
	assert.False(t, created, "segundo unlock deve ser no-op")

	unlocks, err := repo.ListPlayerUnlocks(ctx, "player-1")
	require.NoError(t, err)
	require.Len(t, unlocks, 1)
	assert.WithinDuration(t, at1, unlocks[0].UnlockedAt, time.Second, "unlocked_at deve permanecer o original")
}

func TestUnlock_Guest_PlayerIDTexto(t *testing.T) {
	repo := NewPostgresAchievementRepository(newTestPool(t))
	created, err := repo.Unlock(context.Background(), "guest-abc-123", "first_blood", time.Now().UTC())
	require.NoError(t, err)
	assert.True(t, created)
}

func TestListPlayerUnlocks_IsolaPorJogador(t *testing.T) {
	repo := NewPostgresAchievementRepository(newTestPool(t))
	ctx := context.Background()

	_, err := repo.Unlock(ctx, "player-1", "first_win", time.Now().UTC())
	require.NoError(t, err)

	got, err := repo.ListPlayerUnlocks(ctx, "player-2")
	require.NoError(t, err)
	assert.Empty(t, got, "player-2 não tem desbloqueios")
}
