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

	"github.com/vitormartins1/go-game-backend/services/achievements-service/internal/domain"
)

// sandbox: jogo default do backfill da migração 003 (os seeds ficam sob ele).
const sandbox = "00000000-0000-0000-0000-000000000000"

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
			"../../migrations/003_add_game_scope.sql",
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

func TestListCatalog_SeedMigraParaSandbox(t *testing.T) {
	repo := NewPostgresAchievementRepository(newTestPool(t))
	cat, err := repo.ListCatalog(context.Background(), sandbox)
	require.NoError(t, err)

	byID := map[string]int{}
	for _, a := range cat {
		assert.Equal(t, sandbox, a.GameID)
		byID[a.ID] = a.Target
	}
	assert.Equal(t, 1, byID["first_win"], "first_win target=1 sob sandbox")
	assert.Equal(t, 10, byID["ten_wins"], "ten_wins target=10 sob sandbox")
}

func TestRecordProgress_AtingeMeta_DesbloqueiaETrava(t *testing.T) {
	repo := NewPostgresAchievementRepository(newTestPool(t))
	ctx := context.Background()
	now := time.Now().UTC()

	for i := 1; i < 10; i++ {
		ju, err := repo.RecordProgress(ctx, sandbox, "p1", "ten_wins", 10, now)
		require.NoError(t, err)
		assert.False(t, ju, "ocorrência %d não deve desbloquear", i)
	}
	ju, err := repo.RecordProgress(ctx, sandbox, "p1", "ten_wins", 10, now)
	require.NoError(t, err)
	assert.True(t, ju, "10ª ocorrência desbloqueia")
	ju, err = repo.RecordProgress(ctx, sandbox, "p1", "ten_wins", 10, now)
	require.NoError(t, err)
	assert.False(t, ju, "após desbloqueada é no-op")

	prog := mustProgress(t, repo, sandbox, "p1")
	assert.Equal(t, 10, prog["ten_wins"].Progress, "progress trava em target")
	assert.False(t, prog["ten_wins"].UnlockedAt.IsZero(), "fica desbloqueada")
}

func TestRecordProgress_Target1_DesbloqueiaNoPrimeiro(t *testing.T) {
	repo := NewPostgresAchievementRepository(newTestPool(t))
	ctx := context.Background()

	ju, err := repo.RecordProgress(ctx, sandbox, "p1", "first_win", 1, time.Now().UTC())
	require.NoError(t, err)
	assert.True(t, ju, "target=1 desbloqueia na 1ª")

	ju, err = repo.RecordProgress(ctx, sandbox, "p1", "first_win", 1, time.Now().UTC())
	require.NoError(t, err)
	assert.False(t, ju, "2ª é no-op")
}

func TestCreateAchievement_MesmoIdEmJogosDiferentes_Coexiste(t *testing.T) {
	repo := NewPostgresAchievementRepository(newTestPool(t))
	ctx := context.Background()

	a := &domain.Achievement{GameID: "game-a", ID: "solo", Name: "Solo", Description: "x", EventType: "kill", Target: 1}
	b := &domain.Achievement{GameID: "game-b", ID: "solo", Name: "Solo", Description: "x", EventType: "kill", Target: 1}
	require.NoError(t, repo.CreateAchievement(ctx, a))
	require.NoError(t, repo.CreateAchievement(ctx, b), "mesmo id em outro jogo coexiste")

	// duplicado no MESMO jogo -> ErrAchievementExists
	dup := &domain.Achievement{GameID: "game-a", ID: "solo", Name: "Solo2", EventType: "kill", Target: 1}
	assert.ErrorIs(t, repo.CreateAchievement(ctx, dup), domain.ErrAchievementExists)
}

func TestIsolamentoEntreJogos(t *testing.T) {
	repo := NewPostgresAchievementRepository(newTestPool(t))
	ctx := context.Background()
	require.NoError(t, repo.CreateAchievement(ctx, &domain.Achievement{
		GameID: "game-a", ID: "win", Name: "Win", EventType: "match_won", Target: 1,
	}))

	// catálogo de game-a tem "win"; o de sandbox não.
	catA, err := repo.ListCatalog(ctx, "game-a")
	require.NoError(t, err)
	require.Len(t, catA, 1)
	assert.Equal(t, "win", catA[0].ID)

	catSandbox, err := repo.ListCatalog(ctx, sandbox)
	require.NoError(t, err)
	for _, a := range catSandbox {
		assert.NotEqual(t, "win", a.ID, "win não vaza para o sandbox")
	}

	// progresso em game-a não aparece em outro jogo.
	_, err = repo.RecordProgress(ctx, "game-a", "p1", "win", 1, time.Now().UTC())
	require.NoError(t, err)
	progOutro, err := repo.ListPlayerProgress(ctx, sandbox, "p1")
	require.NoError(t, err)
	assert.Empty(t, progOutro, "progresso é isolado por jogo")
}

func mustProgress(t *testing.T, repo *PostgresAchievementRepository, gameID, playerID string) map[string]struct {
	Progress   int
	UnlockedAt time.Time
} {
	t.Helper()
	list, err := repo.ListPlayerProgress(context.Background(), gameID, playerID)
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

// gameCRUD: jogo próprio dos testes de CRUD, sem os seeds do sandbox.
const gameCRUD = "game-crud"

func TestUpdateAchievement_AlteraCampos(t *testing.T) {
	repo := NewPostgresAchievementRepository(newTestPool(t))
	ctx := context.Background()
	require.NoError(t, repo.CreateAchievement(ctx, &domain.Achievement{
		GameID: gameCRUD, ID: "first_win", Name: "Antigo", Description: "d", EventType: "match_won", Target: 1}))

	err := repo.UpdateAchievement(ctx, &domain.Achievement{
		GameID: gameCRUD, ID: "first_win", Name: "Novo", Description: "d2", EventType: "match_played", Target: 7})
	require.NoError(t, err)

	cat, err := repo.ListCatalog(ctx, gameCRUD)
	require.NoError(t, err)
	require.Len(t, cat, 1)
	assert.Equal(t, "Novo", cat[0].Name)
	assert.Equal(t, "match_played", cat[0].EventType)
	assert.Equal(t, 7, cat[0].Target)
}

func TestUpdateAchievement_Inexistente_ErrNotFound(t *testing.T) {
	repo := NewPostgresAchievementRepository(newTestPool(t))
	err := repo.UpdateAchievement(context.Background(), &domain.Achievement{
		GameID: sandbox, ID: "fantasma", Name: "X", EventType: "e", Target: 1})
	assert.ErrorIs(t, err, domain.ErrAchievementNotFound)
}

func TestDeleteAchievement_RemoveConquistaEProgresso(t *testing.T) {
	repo := NewPostgresAchievementRepository(newTestPool(t))
	ctx := context.Background()
	require.NoError(t, repo.CreateAchievement(ctx, &domain.Achievement{
		GameID: gameCRUD, ID: "first_win", Name: "P", EventType: "match_won", Target: 1}))
	// gera progresso do jogador (FK player_achievements -> achievements)
	_, err := repo.RecordProgress(ctx, gameCRUD, "p1", "first_win", 1, time.Now().UTC())
	require.NoError(t, err)

	require.NoError(t, repo.DeleteAchievement(ctx, gameCRUD, "first_win"))

	cat, err := repo.ListCatalog(ctx, gameCRUD)
	require.NoError(t, err)
	assert.Empty(t, cat, "catálogo vazio após delete")
	prog, err := repo.ListPlayerProgress(ctx, gameCRUD, "p1")
	require.NoError(t, err)
	assert.Empty(t, prog, "progresso do jogador também removido (transação)")
}

func TestDeleteAchievement_Inexistente_ErrNotFound(t *testing.T) {
	repo := NewPostgresAchievementRepository(newTestPool(t))
	err := repo.DeleteAchievement(context.Background(), sandbox, "fantasma")
	assert.ErrorIs(t, err, domain.ErrAchievementNotFound)
}
