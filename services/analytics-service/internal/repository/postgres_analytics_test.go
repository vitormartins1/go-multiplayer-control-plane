//go:build integration

// Testes de integração com Postgres real (testcontainers). Rodar com:
//
//	go test -tags=integration ./services/analytics-service/internal/repository/
//
// Requer Docker em execução.
package repository

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/vitormartins1/go-game-backend/services/analytics-service/internal/domain"
)

func newTestRepo(t *testing.T) *PostgresRepository {
	t.Helper()
	ctx := context.Background()
	ctr, err := tcpostgres.Run(ctx, "postgres:16",
		tcpostgres.WithDatabase("analytics"),
		tcpostgres.WithUsername("dev"),
		tcpostgres.WithPassword("dev"),
		tcpostgres.WithInitScripts("../../migrations/001_create_analytics.sql"),
		tcpostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return New(pool)
}

func match(matchID, gameID string, players ...domain.PlayerResult) domain.MatchResult {
	return domain.MatchResult{MatchID: matchID, GameID: gameID, Results: players}
}

func TestApplyMatch_AgregaRollupEDaily(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	dup, err := repo.ApplyMatch(ctx, match("m1", "g1",
		domain.PlayerResult{PlayerID: "p1", Won: true, Score: 10},
		domain.PlayerResult{PlayerID: "p2", Won: false, Score: 4},
	))
	require.NoError(t, err)
	assert.False(t, dup)

	ga, err := repo.GameAnalytics(ctx, "g1", 30)
	require.NoError(t, err)
	assert.Equal(t, int64(1), ga.Matches)
	assert.Equal(t, int64(2), ga.Participations)
	assert.Equal(t, int64(1), ga.Wins)
	assert.Equal(t, int64(2), ga.UniquePlayers)
	assert.Equal(t, 7.0, ga.AvgScore) // (10+4)/2
	require.Len(t, ga.Daily, 1)
	assert.Equal(t, int64(1), ga.Daily[0].Matches)
}

func TestApplyMatch_Reprocessada_NaoDobra(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	m := match("m1", "g1", domain.PlayerResult{PlayerID: "p1", Won: true, Score: 5})

	dup, err := repo.ApplyMatch(ctx, m)
	require.NoError(t, err)
	assert.False(t, dup)

	dup, err = repo.ApplyMatch(ctx, m) // reentrega at-least-once
	require.NoError(t, err)
	assert.True(t, dup, "duplicata detectada")

	ga, err := repo.GameAnalytics(ctx, "g1", 30)
	require.NoError(t, err)
	assert.Equal(t, int64(1), ga.Matches, "não recontabiliza")
	assert.Equal(t, int64(1), ga.Participations)
}

func TestApplyMatch_JogadorUnicoEmDuasPartidas(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	require.NoErrorf(t, mustApply(repo, ctx, match("m1", "g1", domain.PlayerResult{PlayerID: "p1", Score: 3})), "m1")
	require.NoErrorf(t, mustApply(repo, ctx, match("m2", "g1", domain.PlayerResult{PlayerID: "p1", Score: 7})), "m2")

	ga, err := repo.GameAnalytics(ctx, "g1", 30)
	require.NoError(t, err)
	assert.Equal(t, int64(2), ga.Matches)
	assert.Equal(t, int64(2), ga.Participations)
	assert.Equal(t, int64(1), ga.UniquePlayers, "mesmo player em 2 partidas = 1 único")
}

func TestOverview_OrdenaPorMatchesESomaTotais(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	// g2 tem 2 partidas, g1 tem 1 -> g2 primeiro.
	require.NoError(t, mustApply(repo, ctx, match("m1", "g1", domain.PlayerResult{PlayerID: "p1"})))
	require.NoError(t, mustApply(repo, ctx, match("m2", "g2", domain.PlayerResult{PlayerID: "p1"})))
	require.NoError(t, mustApply(repo, ctx, match("m3", "g2", domain.PlayerResult{PlayerID: "p2"})))

	ov, err := repo.Overview(ctx)
	require.NoError(t, err)
	require.Len(t, ov.Games, 2)
	assert.Equal(t, "g2", ov.Games[0].GameID, "ordena por matches desc")
	assert.Equal(t, int64(3), ov.Totals.Matches)
	assert.Equal(t, int64(2), ov.Totals.Games)
	assert.Equal(t, int64(2), ov.Totals.UniquePlayers, "p1 e p2 distintos globais")
}

func mustApply(repo *PostgresRepository, ctx context.Context, m domain.MatchResult) error {
	_, err := repo.ApplyMatch(ctx, m)
	return err
}
