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
		tcpostgres.WithInitScripts(
			"../../migrations/001_create_achievements.sql",
			"../../migrations/002_add_progress.sql",
		),
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

func TestListCatalog_CarregaSeedComTarget(t *testing.T) {
	repo := NewPostgresAchievementRepository(newTestPool(t))
	cat, err := repo.ListCatalog(context.Background())
	require.NoError(t, err)

	byID := map[string]int{}
	for _, a := range cat {
		byID[a.ID] = a.Target
	}
	assert.Equal(t, 1, byID["first_win"], "first_win target=1")
	assert.Equal(t, 10, byID["ten_wins"], "ten_wins target=10")
}

func TestRecordProgress_AtingeMeta_DesbloqueiaETrava(t *testing.T) {
	repo := NewPostgresAchievementRepository(newTestPool(t))
	ctx := context.Background()
	now := time.Now().UTC()

	// 9 primeiras ocorrências: progride sem desbloquear
	for i := 1; i < 10; i++ {
		ju, err := repo.RecordProgress(ctx, "p1", "ten_wins", 10, now)
		require.NoError(t, err)
		assert.False(t, ju, "ocorrência %d não deve desbloquear", i)
	}
	// 10ª: desbloqueia
	ju, err := repo.RecordProgress(ctx, "p1", "ten_wins", 10, now)
	require.NoError(t, err)
	assert.True(t, ju, "10ª ocorrência desbloqueia")
	// 11ª: no-op
	ju, err = repo.RecordProgress(ctx, "p1", "ten_wins", 10, now)
	require.NoError(t, err)
	assert.False(t, ju, "após desbloqueada é no-op")

	prog := mustProgress(t, repo, "p1")
	assert.Equal(t, 10, prog["ten_wins"].Progress, "progress trava em target")
	assert.False(t, prog["ten_wins"].UnlockedAt.IsZero(), "fica desbloqueada")
}

func TestRecordProgress_Target1_DesbloqueiaNoPrimeiro(t *testing.T) {
	repo := NewPostgresAchievementRepository(newTestPool(t))
	ctx := context.Background()

	ju, err := repo.RecordProgress(ctx, "p1", "first_win", 1, time.Now().UTC())
	require.NoError(t, err)
	assert.True(t, ju, "target=1 desbloqueia na 1ª")

	ju, err = repo.RecordProgress(ctx, "p1", "first_win", 1, time.Now().UTC())
	require.NoError(t, err)
	assert.False(t, ju, "2ª é no-op")
}

func TestRecordProgress_PreservaUnlockedAt(t *testing.T) {
	repo := NewPostgresAchievementRepository(newTestPool(t))
	ctx := context.Background()

	at1 := time.Now().UTC()
	_, err := repo.RecordProgress(ctx, "p1", "first_win", 1, at1)
	require.NoError(t, err)

	at2 := at1.Add(time.Hour)
	_, err = repo.RecordProgress(ctx, "p1", "first_win", 1, at2)
	require.NoError(t, err)

	prog := mustProgress(t, repo, "p1")
	assert.WithinDuration(t, at1, prog["first_win"].UnlockedAt, time.Second, "unlocked_at imutável")
}

func TestListPlayerProgress_IsolaPorJogador(t *testing.T) {
	repo := NewPostgresAchievementRepository(newTestPool(t))
	ctx := context.Background()

	_, err := repo.RecordProgress(ctx, "p1", "ten_wins", 10, time.Now().UTC())
	require.NoError(t, err)

	prog, err := repo.ListPlayerProgress(ctx, "p2")
	require.NoError(t, err)
	assert.Empty(t, prog)
}

// mustProgress indexa o progresso do jogador por achievement_id.
func mustProgress(t *testing.T, repo *PostgresAchievementRepository, playerID string) map[string]struct {
	Progress   int
	UnlockedAt time.Time
} {
	t.Helper()
	list, err := repo.ListPlayerProgress(context.Background(), playerID)
	require.NoError(t, err)
	out := map[string]struct {
		Progress   int
		UnlockedAt time.Time
	}{}
	for _, p := range list {
		out[p.AchievementID] = struct {
			Progress   int
			UnlockedAt time.Time
		}{p.Progress, p.UnlockedAt}
	}
	return out
}
