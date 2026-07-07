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

	"github.com/vitormartins1/go-multiplayer-control-plane/services/analytics-service/internal/domain"
)

func newTestRepo(t *testing.T) *PostgresRepository {
	t.Helper()
	ctx := context.Background()
	ctr, err := tcpostgres.Run(ctx, "postgres:16",
		tcpostgres.WithDatabase("analytics"),
		tcpostgres.WithUsername("dev"),
		tcpostgres.WithPassword("dev"),
		tcpostgres.WithInitScripts(
			"../../migrations/001_create_analytics.sql",
			"../../migrations/002_outcome_counts.sql",
			"../../migrations/003_participation_rollup.sql",
			"../../migrations/004_placement_rollup.sql",
			"../../migrations/005_mission_rollup.sql",
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
	return New(pool)
}

func match(matchID, gameID string, players ...domain.PlayerResult) domain.MatchResult {
	return domain.MatchResult{MatchID: matchID, GameID: gameID, Results: players}
}

func pevt(gameID, sessionID, playerID, role, event string) domain.ParticipationEvent {
	return domain.ParticipationEvent{GameID: gameID, SessionID: sessionID, PlayerID: playerID, Role: role, Event: event, Outcome: "NEUTRAL"}
}

func TestApplyParticipation_AgregaPorPapelEEvento(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	_, err := repo.ApplyParticipation(ctx, pevt("g1", "w1", "inv-1", "invader", "HOST_KILLED"))
	require.NoError(t, err)
	_, err = repo.ApplyParticipation(ctx, pevt("g1", "w2", "inv-2", "invader", "HOST_KILLED"))
	require.NoError(t, err)
	_, err = repo.ApplyParticipation(ctx, pevt("g1", "w1", "coop-1", "coop", "BOSS_DEFEATED"))
	require.NoError(t, err)

	ga, err := repo.GameAnalytics(ctx, "g1", 30)
	require.NoError(t, err)
	require.Len(t, ga.RoleParticipations, 2)
	// ordenado por role, event: coop/BOSS_DEFEATED, invader/HOST_KILLED.
	assert.Equal(t, domain.ParticipationStat{Role: "coop", Event: "BOSS_DEFEATED", Count: 1}, ga.RoleParticipations[0])
	assert.Equal(t, domain.ParticipationStat{Role: "invader", Event: "HOST_KILLED", Count: 2}, ga.RoleParticipations[1])
}

// placement monta um PlacementResult a partir de pares (placement, survivedMs).
func placement(matchID, gameID string, entries ...domain.PlacementEntry) domain.PlacementResult {
	return domain.PlacementResult{MatchID: matchID, GameID: gameID, Entries: entries}
}

func TestApplyPlacement_AgregaHistogramaESobrevivencia(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	// 12 participantes: 1º (winner), 2..10 (top_n = 9), 11 e 12 (rest = 2).
	entries := make([]domain.PlacementEntry, 0, 12)
	for p := 1; p <= 12; p++ {
		entries = append(entries, domain.PlacementEntry{Placement: p, SurvivedMs: int64(p) * 1000})
	}
	_, err := repo.ApplyPlacement(ctx, placement("m1", "g1", entries...))
	require.NoError(t, err)

	ga, err := repo.GameAnalytics(ctx, "g1", 30)
	require.NoError(t, err)
	require.NotNil(t, ga.Placement)
	assert.Equal(t, int64(1), ga.Placement.Winners)
	assert.Equal(t, int64(9), ga.Placement.TopN)
	assert.Equal(t, int64(2), ga.Placement.Rest)
	assert.Equal(t, int64(12), ga.Placement.Entries, "partição fecha: 1+9+2 == 12")
	// avg = (1000+2000+...+12000)/12 = 78000/12 = 6500.
	assert.InDelta(t, 6500.0, ga.Placement.AvgSurvivedMs, 0.001)
}

func TestApplyPlacement_Reprocessado_NaoDobra(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	pr := placement("m1", "g1", domain.PlacementEntry{Placement: 1, SurvivedMs: 500}, domain.PlacementEntry{Placement: 5, SurvivedMs: 300})

	dup, err := repo.ApplyPlacement(ctx, pr)
	require.NoError(t, err)
	assert.False(t, dup)
	dup, err = repo.ApplyPlacement(ctx, pr) // reentrega at-least-once
	require.NoError(t, err)
	assert.True(t, dup)

	ga, err := repo.GameAnalytics(ctx, "g1", 30)
	require.NoError(t, err)
	require.NotNil(t, ga.Placement)
	assert.Equal(t, int64(2), ga.Placement.Entries, "reentrega não dobra")
	assert.Equal(t, int64(1), ga.Placement.Winners)
	assert.Equal(t, int64(1), ga.Placement.TopN)
}

// Jogo sem placement (só queue_arena) não tem distribuição — Placement nil.
func TestGameAnalytics_SemPlacement_PlacementNil(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	_, err := repo.ApplyMatch(ctx, match("m1", "g1", domain.PlayerResult{PlayerID: "p1", Outcome: "win", Score: 10}))
	require.NoError(t, err)

	ga, err := repo.GameAnalytics(ctx, "g1", 30)
	require.NoError(t, err)
	assert.Nil(t, ga.Placement, "jogo não-BR não regride")
	assert.Nil(t, ga.Mission, "jogo não-coop não regride")
}

// coop_mission (C3): sucessos/falhas agregam por jogo; successRate derivado.
func TestApplyCollective_AgregaSucessosEFalhas(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	_, err := repo.ApplyCollective(ctx, domain.CollectiveResult{MatchID: "m1", GameID: "g1", Outcome: domain.MissionSuccess})
	require.NoError(t, err)
	_, err = repo.ApplyCollective(ctx, domain.CollectiveResult{MatchID: "m2", GameID: "g1", Outcome: domain.MissionSuccess})
	require.NoError(t, err)
	_, err = repo.ApplyCollective(ctx, domain.CollectiveResult{MatchID: "m3", GameID: "g1", Outcome: domain.MissionFailure})
	require.NoError(t, err)

	ga, err := repo.GameAnalytics(ctx, "g1", 30)
	require.NoError(t, err)
	require.NotNil(t, ga.Mission)
	assert.Equal(t, int64(2), ga.Mission.Successes)
	assert.Equal(t, int64(1), ga.Mission.Failures)
	assert.InDelta(t, 2.0/3.0, ga.Mission.SuccessRate, 0.001)
}

func TestApplyCollective_Reprocessado_NaoDobra(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	cr := domain.CollectiveResult{MatchID: "m1", GameID: "g1", Outcome: domain.MissionSuccess}

	dup, err := repo.ApplyCollective(ctx, cr)
	require.NoError(t, err)
	assert.False(t, dup)
	dup, err = repo.ApplyCollective(ctx, cr) // reentrega at-least-once
	require.NoError(t, err)
	assert.True(t, dup)

	ga, err := repo.GameAnalytics(ctx, "g1", 30)
	require.NoError(t, err)
	require.NotNil(t, ga.Mission)
	assert.Equal(t, int64(1), ga.Mission.Successes, "reentrega não dobra")
}

func TestApplyParticipation_Reprocessado_NaoDobra(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	evt := pevt("g1", "w1", "inv-1", "invader", "HOST_KILLED")

	dup, err := repo.ApplyParticipation(ctx, evt)
	require.NoError(t, err)
	assert.False(t, dup)
	dup, err = repo.ApplyParticipation(ctx, evt) // reentrega at-least-once
	require.NoError(t, err)
	assert.True(t, dup)

	ga, err := repo.GameAnalytics(ctx, "g1", 30)
	require.NoError(t, err)
	require.Len(t, ga.RoleParticipations, 1)
	assert.Equal(t, int64(1), ga.RoleParticipations[0].Count, "reentrega não dobra")
}

func TestGameAnalytics_QueueArena_SemParticipacoes(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	_, err := repo.ApplyMatch(ctx, match("m1", "g1", domain.PlayerResult{PlayerID: "p1", Outcome: "win"}))
	require.NoError(t, err)

	ga, err := repo.GameAnalytics(ctx, "g1", 30)
	require.NoError(t, err)
	assert.Empty(t, ga.RoleParticipations, "jogo sem host_world não tem participações")
}

func TestApplyMatch_AgregaRollupEDaily(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	dup, err := repo.ApplyMatch(ctx, match("m1", "g1",
		domain.PlayerResult{PlayerID: "p1", Outcome: domain.OutcomeWin, Won: true, Score: 10},
		domain.PlayerResult{PlayerID: "p2", Outcome: domain.OutcomeLoss, Won: false, Score: 4},
	))
	require.NoError(t, err)
	assert.False(t, dup)

	ga, err := repo.GameAnalytics(ctx, "g1", 30)
	require.NoError(t, err)
	assert.Equal(t, int64(1), ga.Matches)
	assert.Equal(t, int64(2), ga.Participations)
	assert.Equal(t, int64(1), ga.Wins)
	assert.Equal(t, int64(1), ga.Losses)
	assert.Equal(t, int64(2), ga.UniquePlayers)
	assert.Equal(t, 7.0, ga.AvgScore) // (10+4)/2
	require.Len(t, ga.Daily, 1)
	assert.Equal(t, int64(1), ga.Daily[0].Matches)
}

// win/loss/draw agregam em contadores distintos; empate ≠ derrota; a soma dos
// três iguala as participações (ADR-0023).
func TestApplyMatch_WinLossDraw_ContadoresDistintos(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	_, err := repo.ApplyMatch(ctx, match("m1", "g1",
		domain.PlayerResult{PlayerID: "p1", Outcome: domain.OutcomeWin, Score: 5},
		domain.PlayerResult{PlayerID: "p2", Outcome: domain.OutcomeLoss, Score: 2},
		domain.PlayerResult{PlayerID: "p3", Outcome: domain.OutcomeDraw, Score: 3},
		domain.PlayerResult{PlayerID: "p4", Outcome: domain.OutcomeDraw, Score: 3},
	))
	require.NoError(t, err)

	ga, err := repo.GameAnalytics(ctx, "g1", 30)
	require.NoError(t, err)
	assert.Equal(t, int64(1), ga.Wins)
	assert.Equal(t, int64(1), ga.Losses)
	assert.Equal(t, int64(2), ga.Draws, "empate contado à parte, não como derrota")
	assert.Equal(t, ga.Participations, ga.Wins+ga.Losses+ga.Draws)
}

func TestApplyMatch_Reprocessada_NaoDobra(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	m := match("m1", "g1", domain.PlayerResult{PlayerID: "p1", Outcome: domain.OutcomeWin, Won: true, Score: 5})

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

	// série diária cross-game: as 3 partidas caem hoje -> 1 ponto com 3.
	require.Len(t, ov.Daily, 1)
	assert.Equal(t, int64(3), ov.Daily[0].Matches, "soma partidas/dia entre jogos")
}

func mustApply(repo *PostgresRepository, ctx context.Context, m domain.MatchResult) error {
	_, err := repo.ApplyMatch(ctx, m)
	return err
}
