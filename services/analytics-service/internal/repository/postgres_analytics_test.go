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
	"time"

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
			"../../migrations/006_participation_event_id_and_outcome.sql",
			"../../migrations/007_participation_mode_id.sql",
			"../../migrations/008_archetype_daily.sql",
			"../../migrations/009_player_stats.sql",
			"../../migrations/010_session_health.sql",
			"../../migrations/011_mode_scoped_cumulative_rollups.sql",
			"../../migrations/012_session_closed_daily.sql",
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

// matchWithMode (spec 012) — mesma forma de match, com ModeID explícito
// (ausente em match(): dado legado "sem modo").
func matchWithMode(matchID, gameID, modeID string, players ...domain.PlayerResult) domain.MatchResult {
	return domain.MatchResult{MatchID: matchID, GameID: gameID, ModeID: modeID, Results: players}
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

	ga, err := repo.GameAnalytics(ctx, "g1", 30, "")
	require.NoError(t, err)
	require.Len(t, ga.RoleParticipations, 2)
	// ordenado por role, event, outcome: coop/BOSS_DEFEATED, invader/HOST_KILLED.
	assert.Equal(t, domain.ParticipationStat{Role: "coop", Event: "BOSS_DEFEATED", Outcome: "NEUTRAL", Count: 1}, ga.RoleParticipations[0])
	assert.Equal(t, domain.ParticipationStat{Role: "invader", Event: "HOST_KILLED", Outcome: "NEUTRAL", Count: 2}, ga.RoleParticipations[1])
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

	ga, err := repo.GameAnalytics(ctx, "g1", 30, "")
	require.NoError(t, err)
	require.NotNil(t, ga.Placement)
	assert.Equal(t, int64(1), ga.Placement.Winners)
	assert.Equal(t, int64(9), ga.Placement.TopN)
	assert.Equal(t, int64(2), ga.Placement.Rest)
	assert.Equal(t, int64(12), ga.Placement.Entries, "partição fecha: 1+9+2 == 12")
	// avg = (1000+2000+...+12000)/12 = 78000/12 = 6500.
	assert.InDelta(t, 6500.0, ga.Placement.AvgSurvivedMs, 0.001)
}

// série diária por modo (spec 010) — battle_royale não tinha antes.
func TestApplyPlacement_GravaSerieDiaria(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	pr := placement("m1", "g1", domain.PlacementEntry{Placement: 1, SurvivedMs: 500})
	pr.ModeID = "batizado"
	_, err := repo.ApplyPlacement(ctx, pr)
	require.NoError(t, err)

	ga, err := repo.GameAnalytics(ctx, "g1", 30, "")
	require.NoError(t, err)
	require.Len(t, ga.PlacementDaily, 1)
	assert.Equal(t, int64(1), ga.PlacementDaily[0].Matches)
	assert.Empty(t, ga.MissionDaily, "sem missão -> vazio")
	assert.Empty(t, ga.ParticipationDaily, "sem participação -> vazio")
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

	ga, err := repo.GameAnalytics(ctx, "g1", 30, "")
	require.NoError(t, err)
	require.NotNil(t, ga.Placement)
	assert.Equal(t, int64(2), ga.Placement.Entries, "reentrega não dobra")
	assert.Equal(t, int64(1), ga.Placement.Winners)
	assert.Equal(t, int64(1), ga.Placement.TopN)
	require.Len(t, ga.PlacementDaily, 1)
	assert.Equal(t, int64(1), ga.PlacementDaily[0].Matches, "reentrega não dobra a série diária")
}

// Jogo sem placement (só queue_arena) não tem distribuição — Placement nil.
func TestGameAnalytics_SemPlacement_PlacementNil(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	_, err := repo.ApplyMatch(ctx, match("m1", "g1", domain.PlayerResult{PlayerID: "p1", Outcome: "win", Score: 10}))
	require.NoError(t, err)

	ga, err := repo.GameAnalytics(ctx, "g1", 30, "")
	require.NoError(t, err)
	assert.Nil(t, ga.Placement, "jogo não-BR não regride")
	assert.Nil(t, ga.Mission, "jogo não-coop não regride")
	assert.Empty(t, ga.PlacementDaily)
	assert.Empty(t, ga.MissionDaily)
	assert.Empty(t, ga.ParticipationDaily)
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

	ga, err := repo.GameAnalytics(ctx, "g1", 30, "")
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

	ga, err := repo.GameAnalytics(ctx, "g1", 30, "")
	require.NoError(t, err)
	require.NotNil(t, ga.Mission)
	assert.Equal(t, int64(1), ga.Mission.Successes, "reentrega não dobra")
	require.Len(t, ga.MissionDaily, 1)
	assert.Equal(t, int64(1), ga.MissionDaily[0].Matches, "reentrega não dobra a série diária")
}

// série diária por modo (spec 010) — coop_mission não tinha antes.
func TestApplyCollective_GravaSerieDiaria(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	_, err := repo.ApplyCollective(ctx, domain.CollectiveResult{MatchID: "m1", GameID: "g1", ModeID: "default", Outcome: domain.MissionSuccess})
	require.NoError(t, err)

	ga, err := repo.GameAnalytics(ctx, "g1", 30, "")
	require.NoError(t, err)
	require.Len(t, ga.MissionDaily, 1)
	assert.Equal(t, int64(1), ga.MissionDaily[0].Matches)
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

	ga, err := repo.GameAnalytics(ctx, "g1", 30, "")
	require.NoError(t, err)
	require.Len(t, ga.RoleParticipations, 1)
	assert.Equal(t, int64(1), ga.RoleParticipations[0].Count, "reentrega não dobra")
	require.Len(t, ga.ParticipationDaily, 1)
	assert.Equal(t, int64(1), ga.ParticipationDaily[0].Matches, "reentrega não dobra a série diária")
}

// série diária por modo (spec 010) — host_world_dropin não tinha antes;
// conta EVENTOS (não sessões), acumulando entre eventos distintos no dia.
func TestApplyParticipation_GravaSerieDiaria(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	_, err := repo.ApplyParticipation(ctx, pevt("g1", "w1", "coop-1", "coop", "JOINED"))
	require.NoError(t, err)
	_, err = repo.ApplyParticipation(ctx, pevt("g1", "w1", "coop-1", "coop", "BOSS_DEFEATED"))
	require.NoError(t, err)

	ga, err := repo.GameAnalytics(ctx, "g1", 30, "")
	require.NoError(t, err)
	require.Len(t, ga.ParticipationDaily, 1)
	assert.Equal(t, int64(2), ga.ParticipationDaily[0].Matches, "2 eventos distintos no mesmo dia somam")
}

// Dois eventId diferentes (mesma sessão/jogador/evento) contam duas vezes —
// o cenário do follow-up #4 (ex.: boss que respawna).
func TestApplyParticipation_EventIDsDiferentes_ContaDuasVezes(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	evtA := pevt("g1", "w1", "coop-1", "coop", "BOSS_DEFEATED")
	evtA.EventID = "evt-a"
	evtB := pevt("g1", "w1", "coop-1", "coop", "BOSS_DEFEATED")
	evtB.EventID = "evt-b"

	_, err := repo.ApplyParticipation(ctx, evtA)
	require.NoError(t, err)
	_, err = repo.ApplyParticipation(ctx, evtB)
	require.NoError(t, err)

	ga, err := repo.GameAnalytics(ctx, "g1", 30, "")
	require.NoError(t, err)
	require.Len(t, ga.RoleParticipations, 1)
	assert.Equal(t, int64(2), ga.RoleParticipations[0].Count, "eventId diferente -> conta as duas ocorrências")
}

// Mesmo (role, event), outcome diferente -> linhas separadas no rollup
// (follow-up #5). Sintético (mesmo papel/evento com dois outcomes não é o
// caso real de HOST_KILLED, mas é exatamente o que a mudança de schema
// precisa suportar — o teste isola a dimensão outcome de role/event).
func TestApplyParticipation_MesmoRoleEvento_OutcomesDiferentes_LinhasSeparadas(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	success := domain.ParticipationEvent{GameID: "g1", SessionID: "w1", PlayerID: "inv-1", Role: "invader", Event: "HOST_KILLED", Outcome: "SUCCESS"}
	fail := domain.ParticipationEvent{GameID: "g1", SessionID: "w2", PlayerID: "inv-2", Role: "invader", Event: "HOST_KILLED", Outcome: "FAIL"}

	_, err := repo.ApplyParticipation(ctx, success)
	require.NoError(t, err)
	_, err = repo.ApplyParticipation(ctx, fail)
	require.NoError(t, err)

	ga, err := repo.GameAnalytics(ctx, "g1", 30, "")
	require.NoError(t, err)
	require.Len(t, ga.RoleParticipations, 2, "mesmo role/event, outcome diferente -> linhas separadas")
	byOutcome := map[string]domain.ParticipationStat{}
	for _, p := range ga.RoleParticipations {
		byOutcome[p.Outcome] = p
	}
	assert.Equal(t, int64(1), byOutcome["SUCCESS"].Count)
	assert.Equal(t, int64(1), byOutcome["FAIL"].Count)
}

// Dois modos nomeados do mesmo jogo (spec 009, ADR-0024) -> linhas separadas
// no rollup; dado sem modo (ModeID="") cai no bucket "sem modo" à parte.
func TestApplyParticipation_ModosDiferentes_LinhasSeparadas(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	rodaAberta := domain.ParticipationEvent{GameID: "g1", ModeID: "default", SessionID: "w1", PlayerID: "coop-1", Role: "coop", Event: "BOSS_DEFEATED", Outcome: "NEUTRAL"}
	batizado := domain.ParticipationEvent{GameID: "g1", ModeID: "batizado", SessionID: "w2", PlayerID: "coop-2", Role: "coop", Event: "BOSS_DEFEATED", Outcome: "NEUTRAL"}
	semModo := domain.ParticipationEvent{GameID: "g1", SessionID: "w3", PlayerID: "coop-3", Role: "coop", Event: "BOSS_DEFEATED", Outcome: "NEUTRAL"} // ModeID="" (dado anterior à spec 009)

	_, err := repo.ApplyParticipation(ctx, rodaAberta)
	require.NoError(t, err)
	_, err = repo.ApplyParticipation(ctx, batizado)
	require.NoError(t, err)
	_, err = repo.ApplyParticipation(ctx, semModo)
	require.NoError(t, err)

	ga, err := repo.GameAnalytics(ctx, "g1", 30, "")
	require.NoError(t, err)
	require.Len(t, ga.RoleParticipations, 3, "3 modos distintos (incl. \"sem modo\") -> 3 linhas, nenhuma soma na outra")
	byMode := map[string]int64{}
	for _, p := range ga.RoleParticipations {
		byMode[p.ModeID] = p.Count
	}
	assert.Equal(t, int64(1), byMode["default"])
	assert.Equal(t, int64(1), byMode["batizado"])
	assert.Equal(t, int64(1), byMode[""], "dado sem modo cai no bucket \"\"")
}

func TestGameAnalytics_QueueArena_SemParticipacoes(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	_, err := repo.ApplyMatch(ctx, match("m1", "g1", domain.PlayerResult{PlayerID: "p1", Outcome: "win"}))
	require.NoError(t, err)

	ga, err := repo.GameAnalytics(ctx, "g1", 30, "")
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

	ga, err := repo.GameAnalytics(ctx, "g1", 30, "")
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

	ga, err := repo.GameAnalytics(ctx, "g1", 30, "")
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

	ga, err := repo.GameAnalytics(ctx, "g1", 30, "")
	require.NoError(t, err)
	assert.Equal(t, int64(1), ga.Matches, "não recontabiliza")
	assert.Equal(t, int64(1), ga.Participations)
}

func TestApplyMatch_JogadorUnicoEmDuasPartidas(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	require.NoErrorf(t, mustApply(repo, ctx, match("m1", "g1", domain.PlayerResult{PlayerID: "p1", Score: 3})), "m1")
	require.NoErrorf(t, mustApply(repo, ctx, match("m2", "g1", domain.PlayerResult{PlayerID: "p1", Score: 7})), "m2")

	ga, err := repo.GameAnalytics(ctx, "g1", 30, "")
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

	// spec 016: a série do overview é de SESSÕES fechadas (nenhuma aqui).
	assert.Empty(t, ov.Daily)
}

// Spec 016: um jogo só de host_world_dropin (o Folclore Souls) aparece no
// overview com sessões, duração média, abandono e jogadores únicos — antes
// sumia (o overview lia só o rollup do queue_arena).
func TestOverview_JogoSoDeMundo_ApareceComSessoesEJogadores(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	base := recentDay()

	_, err := repo.ApplyParticipation(ctx, domain.ParticipationEvent{
		GameID: "souls", ModeID: "default", SessionID: "w1", PlayerID: "inv-1",
		Role: "invader", Event: "HOST_KILLED", Outcome: "SUCCESS", EventID: "e1",
	})
	require.NoError(t, err)
	_, err = repo.ApplySessionClosed(ctx, closeEvent("w1", "souls", "default", "host_world_dropin", "host_killed", base, base.Add(10*time.Minute)))
	require.NoError(t, err)
	_, err = repo.ApplySessionClosed(ctx, closeEvent("w2", "souls", "default", "host_world_dropin", "abandoned", base, base.Add(20*time.Minute)))
	require.NoError(t, err)

	ov, err := repo.Overview(ctx)
	require.NoError(t, err)
	require.Len(t, ov.Games, 1)
	g := ov.Games[0]
	assert.Equal(t, "souls", g.GameID)
	assert.Equal(t, int64(2), g.Sessions)
	assert.Equal(t, int64(15*60*1000), g.AvgSessionMs)
	assert.Equal(t, int64(1), g.Abandoned)
	assert.Equal(t, int64(1), g.UniquePlayers, "participação marca o jogador como visto")
	assert.Equal(t, int64(0), g.Matches)
	assert.Equal(t, int64(2), ov.Totals.Sessions)
	require.Len(t, ov.Daily, 1)
	assert.Equal(t, int64(2), ov.Daily[0].Sessions)
}

func mustApply(repo *PostgresRepository, ctx context.Context, m domain.MatchResult) error {
	_, err := repo.ApplyMatch(ctx, m)
	return err
}

// --- spec 011: captura de stats brutos ---

func TestApplyMatch_GravaStatsBrutos(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	_, err := repo.ApplyMatch(ctx, match("m1", "g1",
		domain.PlayerResult{PlayerID: "p1", Outcome: "win", Score: 10, Stats: map[string]int64{"kills": 7}},
		domain.PlayerResult{PlayerID: "p2", Outcome: "loss", Score: 2, Stats: map[string]int64{"kills": 1}},
	))
	require.NoError(t, err)

	ga, err := repo.GameAnalytics(ctx, "g1", 30, "")
	require.NoError(t, err)
	require.Len(t, ga.Stats, 1)
	assert.Equal(t, "kills", ga.Stats[0].Key)
	assert.Equal(t, int64(8), ga.Stats[0].Sum)
	assert.Equal(t, int64(2), ga.Stats[0].Count)
	assert.InDelta(t, 4.0, ga.Stats[0].Avg, 0.001)
}

func TestApplyMatch_SemStats_NaoGravaNada(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	require.NoError(t, mustApply(repo, ctx, match("m1", "g1", domain.PlayerResult{PlayerID: "p1", Score: 10})))

	ga, err := repo.GameAnalytics(ctx, "g1", 30, "")
	require.NoError(t, err)
	assert.Empty(t, ga.Stats, "jogo sem scoringStats nunca reporta stats")
}

func TestApplyMatch_Reprocessado_NaoDobraStats(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	m := match("m1", "g1", domain.PlayerResult{PlayerID: "p1", Stats: map[string]int64{"kills": 5}})

	require.NoError(t, mustApply(repo, ctx, m))
	require.NoError(t, mustApply(repo, ctx, m)) // reentrega at-least-once

	ga, err := repo.GameAnalytics(ctx, "g1", 30, "")
	require.NoError(t, err)
	require.Len(t, ga.Stats, 1)
	assert.Equal(t, int64(5), ga.Stats[0].Sum, "reentrega não dobra")
	assert.Equal(t, int64(1), ga.Stats[0].Count)
}

func TestApplyPlacement_GravaStatsBrutos(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	pr := placement("m1", "g1",
		domain.PlacementEntry{PlayerID: "p1", Placement: 1, SurvivedMs: 900000, Stats: map[string]int64{"kills": 4}},
		domain.PlacementEntry{PlayerID: "p2", Placement: 2, SurvivedMs: 800000},
	)
	_, err := repo.ApplyPlacement(ctx, pr)
	require.NoError(t, err)

	ga, err := repo.GameAnalytics(ctx, "g1", 30, "")
	require.NoError(t, err)
	require.Len(t, ga.Stats, 1)
	assert.Equal(t, "kills", ga.Stats[0].Key)
	assert.Equal(t, int64(4), ga.Stats[0].Sum)
	assert.Equal(t, int64(1), ga.Stats[0].Count, "só p1 reportou kills")
}

func TestApplyCollective_GravaStatsBrutos(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	cr := domain.CollectiveResult{
		MatchID: "m1", GameID: "g1", Outcome: domain.MissionSuccess,
		Players: []domain.CollectivePlayerStat{
			{PlayerID: "p1", Stats: map[string]int64{"revives": 2}},
			{PlayerID: "p2", Stats: map[string]int64{"revives": 1}},
		},
	}
	_, err := repo.ApplyCollective(ctx, cr)
	require.NoError(t, err)

	ga, err := repo.GameAnalytics(ctx, "g1", 30, "")
	require.NoError(t, err)
	require.Len(t, ga.Stats, 1)
	assert.Equal(t, "revives", ga.Stats[0].Key)
	assert.Equal(t, int64(3), ga.Stats[0].Sum)
	assert.Equal(t, int64(2), ga.Stats[0].Count)
}

func TestApplyParticipation_GravaStatsBrutos(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	evt := pevt("g1", "w1", "coop-1", "coop", "DIED")
	evt.Stats = map[string]int64{"deaths": 1}
	_, err := repo.ApplyParticipation(ctx, evt)
	require.NoError(t, err)

	ga, err := repo.GameAnalytics(ctx, "g1", 30, "")
	require.NoError(t, err)
	require.Len(t, ga.Stats, 1)
	assert.Equal(t, "deaths", ga.Stats[0].Key)
	assert.Equal(t, int64(1), ga.Stats[0].Sum)
}

func TestGameAnalytics_Stats_AgregaEntrePartidasDistintas(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	require.NoError(t, mustApply(repo, ctx, match("m1", "g1", domain.PlayerResult{PlayerID: "p1", Stats: map[string]int64{"kills": 3}})))
	require.NoError(t, mustApply(repo, ctx, match("m2", "g1", domain.PlayerResult{PlayerID: "p2", Stats: map[string]int64{"kills": 5}})))

	ga, err := repo.GameAnalytics(ctx, "g1", 30, "")
	require.NoError(t, err)
	require.Len(t, ga.Stats, 1)
	assert.Equal(t, int64(8), ga.Stats[0].Sum)
	assert.Equal(t, int64(2), ga.Stats[0].Count)
	assert.InDelta(t, 4.0, ga.Stats[0].Avg, 0.001)
}

// TestGameAnalytics_ModeID_GravaEIsolaStatsDoQueueArena (spec 012): o
// achado adicional — ApplyMatch gravava mode_id='' hardcoded, misturando
// "kills" de dois modos distintos do mesmo jogo num único total.
func TestGameAnalytics_ModeID_GravaEIsolaStatsDoQueueArena(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	require.NoError(t, mustApply(repo, ctx, matchWithMode("m1", "g1", "ffa", domain.PlayerResult{PlayerID: "p1", Stats: map[string]int64{"kills": 3}})))
	require.NoError(t, mustApply(repo, ctx, matchWithMode("m2", "g1", "teams-2v2", domain.PlayerResult{PlayerID: "p2", Stats: map[string]int64{"kills": 5}})))

	all, err := repo.GameAnalytics(ctx, "g1", 30, "")
	require.NoError(t, err)
	require.Len(t, all.Stats, 1, "sem filtro: soma os dois modos (mesma semântica das séries diárias)")
	assert.Equal(t, int64(8), all.Stats[0].Sum)

	ffa, err := repo.GameAnalytics(ctx, "g1", 30, "ffa")
	require.NoError(t, err)
	require.Len(t, ffa.Stats, 1)
	assert.Equal(t, int64(3), ffa.Stats[0].Sum, "filtrado por modo: só o modo pedido")
	assert.Equal(t, "ffa", ffa.ModeID, "eco do filtro aplicado")

	teams, err := repo.GameAnalytics(ctx, "g1", 30, "teams-2v2")
	require.NoError(t, err)
	require.Len(t, teams.Stats, 1)
	assert.Equal(t, int64(5), teams.Stats[0].Sum)
}

// TestGameAnalytics_ModeIDFiltro_IsolaSeriesDiarias (spec 012): o filtro de
// modo cobre também as séries diárias dos 3 arquétipos (spec 010), não só stats.
func TestGameAnalytics_ModeIDFiltro_IsolaSeriesDiarias(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	pr1 := placement("m1", "g1", domain.PlacementEntry{Placement: 1})
	pr1.ModeID = "royale-a"
	_, err := repo.ApplyPlacement(ctx, pr1)
	require.NoError(t, err)

	pr2 := placement("m2", "g1", domain.PlacementEntry{Placement: 1})
	pr2.ModeID = "royale-b"
	_, err = repo.ApplyPlacement(ctx, pr2)
	require.NoError(t, err)

	all, err := repo.GameAnalytics(ctx, "g1", 30, "")
	require.NoError(t, err)
	require.Len(t, all.PlacementDaily, 1)
	assert.Equal(t, int64(2), all.PlacementDaily[0].Matches, "sem filtro: soma os 2 modos")

	a, err := repo.GameAnalytics(ctx, "g1", 30, "royale-a")
	require.NoError(t, err)
	require.Len(t, a.PlacementDaily, 1)
	assert.Equal(t, int64(1), a.PlacementDaily[0].Matches, "filtrado: só o modo pedido")
}

// closeEvent monta um SessionCloseEvent com duração derivada de opened/closed.
func closeEvent(sessionID, gameID, modeID, archetype, closeReason string, opened, closed time.Time) domain.SessionCloseEvent {
	return domain.SessionCloseEvent{
		SessionID: sessionID, GameID: gameID, ModeID: modeID, Archetype: archetype,
		CloseReason: closeReason, OpenedAt: opened, ClosedAt: closed,
	}
}

// TestApplySessionClosed_GravaRollupPorMotivo (spec 013): count + duração
// somada por (game, mode, closeReason).
func TestApplySessionClosed_GravaRollupPorMotivo(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	base := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	_, err := repo.ApplySessionClosed(ctx, closeEvent("s1", "g1", "mundo-vivo", "host_world_dropin", "host_left", base, base.Add(6*time.Minute)))
	require.NoError(t, err)
	_, err = repo.ApplySessionClosed(ctx, closeEvent("s2", "g1", "mundo-vivo", "host_world_dropin", "host_left", base, base.Add(4*time.Minute)))
	require.NoError(t, err)
	_, err = repo.ApplySessionClosed(ctx, closeEvent("s3", "g1", "mundo-vivo", "host_world_dropin", "ttl", base, base.Add(30*time.Minute)))
	require.NoError(t, err)

	ga, err := repo.GameAnalytics(ctx, "g1", 30, "")
	require.NoError(t, err)
	require.Len(t, ga.SessionHealth, 2)

	byReason := map[string]domain.SessionCloseStat{}
	for _, h := range ga.SessionHealth {
		byReason[h.CloseReason] = h
	}
	require.Contains(t, byReason, "host_left")
	assert.Equal(t, int64(2), byReason["host_left"].Count)
	assert.Equal(t, int64(10*60*1000), byReason["host_left"].DurationSumMs, "6min + 4min")
	assert.InDelta(t, 5*60*1000, byReason["host_left"].AvgDurationMs, 0.001)
	assert.Equal(t, "host_world_dropin", byReason["host_left"].Archetype)

	require.Contains(t, byReason, "ttl")
	assert.Equal(t, int64(1), byReason["ttl"].Count)
}

func TestApplySessionClosed_Reprocessado_NaoDobra(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	base := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	evt := closeEvent("s1", "g1", "default", "queue_arena", "result", base, base.Add(2*time.Minute))
	dup1, err := repo.ApplySessionClosed(ctx, evt)
	require.NoError(t, err)
	assert.False(t, dup1)

	dup2, err := repo.ApplySessionClosed(ctx, evt)
	require.NoError(t, err)
	assert.True(t, dup2, "reentrega do mesmo sessionId é no-op")

	ga, err := repo.GameAnalytics(ctx, "g1", 30, "")
	require.NoError(t, err)
	require.Len(t, ga.SessionHealth, 1)
	assert.Equal(t, int64(1), ga.SessionHealth[0].Count, "não dobrou")
}

// TestGameAnalytics_SessionHealth_FiltraPorModo (spec 013): a tabela já
// nasce com mode_id, sem a lacuna do item D.
func TestGameAnalytics_SessionHealth_FiltraPorModo(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	base := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	_, err := repo.ApplySessionClosed(ctx, closeEvent("s1", "g1", "modo-a", "queue_arena", "result", base, base.Add(time.Minute)))
	require.NoError(t, err)
	_, err = repo.ApplySessionClosed(ctx, closeEvent("s2", "g1", "modo-b", "queue_arena", "result", base, base.Add(time.Minute)))
	require.NoError(t, err)

	all, err := repo.GameAnalytics(ctx, "g1", 30, "")
	require.NoError(t, err)
	assert.Len(t, all.SessionHealth, 2, "sem filtro: os 2 modos")

	a, err := repo.GameAnalytics(ctx, "g1", 30, "modo-a")
	require.NoError(t, err)
	require.Len(t, a.SessionHealth, 1)
	assert.Equal(t, "modo-a", a.SessionHealth[0].ModeID)
}

// TestGameAnalytics_ModeID_IsolaRollupCumulativo (spec 014): game_rollup/
// game_daily/game_player_seen agora respeitam o filtro de modo — a
// lacuna que a spec 012 documentou como "fora de escopo".
func TestGameAnalytics_ModeID_IsolaRollupCumulativo(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	require.NoError(t, mustApply(repo, ctx, matchWithMode("m1", "g1", "ffa",
		domain.PlayerResult{PlayerID: "p1", Outcome: "win", Score: 10})))
	require.NoError(t, mustApply(repo, ctx, matchWithMode("m2", "g1", "teams-2v2",
		domain.PlayerResult{PlayerID: "p2", Outcome: "loss", Score: 3})))

	all, err := repo.GameAnalytics(ctx, "g1", 30, "")
	require.NoError(t, err)
	assert.Equal(t, int64(2), all.Matches, "sem filtro: soma os 2 modos")
	assert.Equal(t, int64(1), all.Wins)
	assert.Equal(t, int64(1), all.Losses)
	require.Len(t, all.Daily, 1)
	assert.Equal(t, int64(2), all.Daily[0].Matches)
	assert.Equal(t, int64(2), all.UniquePlayers, "p1 e p2 são jogadores distintos")

	ffa, err := repo.GameAnalytics(ctx, "g1", 30, "ffa")
	require.NoError(t, err)
	assert.Equal(t, int64(1), ffa.Matches, "filtrado: só o modo pedido")
	assert.Equal(t, int64(1), ffa.Wins)
	assert.Equal(t, int64(0), ffa.Losses)
	require.Len(t, ffa.Daily, 1)
	assert.Equal(t, int64(1), ffa.Daily[0].Matches)
	assert.Equal(t, int64(1), ffa.UniquePlayers)
}

// TestGameAnalytics_ModeID_JogadorEmDoisModos_UniquePlayersNaoDuplica
// (spec 014): jogador que jogou 2 modos do mesmo jogo conta 1 vez no
// total global e 1 vez em CADA modo (não é duplicata, são fatos distintos).
func TestGameAnalytics_ModeID_JogadorEmDoisModos_UniquePlayersNaoDuplica(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	require.NoError(t, mustApply(repo, ctx, matchWithMode("m1", "g1", "ffa",
		domain.PlayerResult{PlayerID: "p1", Outcome: "win", Score: 1})))
	require.NoError(t, mustApply(repo, ctx, matchWithMode("m2", "g1", "teams-2v2",
		domain.PlayerResult{PlayerID: "p1", Outcome: "win", Score: 1})))

	all, err := repo.GameAnalytics(ctx, "g1", 30, "")
	require.NoError(t, err)
	assert.Equal(t, int64(1), all.UniquePlayers, "mesmo jogador nos 2 modos: 1 no total global")

	ffa, err := repo.GameAnalytics(ctx, "g1", 30, "ffa")
	require.NoError(t, err)
	assert.Equal(t, int64(1), ffa.UniquePlayers)
}

// TestGameAnalytics_ModeID_IsolaPlacementEMission (spec 014):
// game_placement_rollup/game_mission_rollup agora respeitam o filtro.
func TestGameAnalytics_ModeID_IsolaPlacementEMission(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	pr1 := placement("m1", "g1", domain.PlacementEntry{Placement: 1, SurvivedMs: 1000})
	pr1.ModeID = "royale-a"
	_, err := repo.ApplyPlacement(ctx, pr1)
	require.NoError(t, err)
	pr2 := placement("m2", "g1", domain.PlacementEntry{Placement: 1, SurvivedMs: 2000})
	pr2.ModeID = "royale-b"
	_, err = repo.ApplyPlacement(ctx, pr2)
	require.NoError(t, err)

	all, err := repo.GameAnalytics(ctx, "g1", 30, "")
	require.NoError(t, err)
	require.NotNil(t, all.Placement)
	assert.Equal(t, int64(2), all.Placement.Entries, "sem filtro: soma os 2 modos")

	a, err := repo.GameAnalytics(ctx, "g1", 30, "royale-a")
	require.NoError(t, err)
	require.NotNil(t, a.Placement)
	assert.Equal(t, int64(1), a.Placement.Entries)
	assert.InDelta(t, 1000.0, a.Placement.AvgSurvivedMs, 0.001)

	cr1 := domain.CollectiveResult{MatchID: "m3", GameID: "g1", ModeID: "coop-a", Outcome: domain.MissionSuccess}
	_, err = repo.ApplyCollective(ctx, cr1)
	require.NoError(t, err)
	cr2 := domain.CollectiveResult{MatchID: "m4", GameID: "g1", ModeID: "coop-b", Outcome: domain.MissionFailure}
	_, err = repo.ApplyCollective(ctx, cr2)
	require.NoError(t, err)

	allM, err := repo.GameAnalytics(ctx, "g1", 30, "")
	require.NoError(t, err)
	require.NotNil(t, allM.Mission)
	assert.Equal(t, int64(1), allM.Mission.Successes)
	assert.Equal(t, int64(1), allM.Mission.Failures)

	coopA, err := repo.GameAnalytics(ctx, "g1", 30, "coop-a")
	require.NoError(t, err)
	require.NotNil(t, coopA.Mission)
	assert.Equal(t, int64(1), coopA.Mission.Successes)
	assert.Equal(t, int64(0), coopA.Mission.Failures)
}

// TestOverview_JogoComDoisModos_NaoDuplica (spec 014): achado crítico da
// revisão — sem GROUP BY, um jogo com 2+ modos apareceria 2x no board.
func TestOverview_JogoComDoisModos_NaoDuplica(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	require.NoError(t, mustApply(repo, ctx, matchWithMode("m1", "g1", "ffa",
		domain.PlayerResult{PlayerID: "p1", Outcome: "win", Score: 5})))
	require.NoError(t, mustApply(repo, ctx, matchWithMode("m2", "g1", "teams-2v2",
		domain.PlayerResult{PlayerID: "p2", Outcome: "loss", Score: 2})))

	ov, err := repo.Overview(ctx)
	require.NoError(t, err)
	require.Len(t, ov.Games, 1, "jogo com 2 modos aparece UMA vez só")
	assert.Equal(t, "g1", ov.Games[0].GameID)
	assert.Equal(t, int64(2), ov.Games[0].Matches, "totais somados entre os modos")
	assert.Equal(t, int64(2), ov.Totals.UniquePlayers)
}

// recentDay devolve ontem às 10h UTC: dentro da janela de 30 dias da série
// diária e sempre no passado. Os testes da spec 015 usavam a data fixa
// 2026-07-13 e começaram a falhar sozinhos quando ela saiu da janela.
func recentDay() time.Time {
	y := time.Now().UTC().AddDate(0, 0, -1)
	return time.Date(y.Year(), y.Month(), y.Day(), 10, 0, 0, 0, time.UTC)
}

// TestApplySessionClosed_GravaSerieDiaria (spec 015): sessões fechadas
// grava um ponto no dia (derivado de ClosedAt), soma no mesmo dia.
func TestApplySessionClosed_GravaSerieDiaria(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	base := recentDay()
	_, err := repo.ApplySessionClosed(ctx, closeEvent("s1", "g1", "default", "queue_arena", "result", base, base.Add(time.Minute)))
	require.NoError(t, err)
	_, err = repo.ApplySessionClosed(ctx, closeEvent("s2", "g1", "default", "queue_arena", "result", base, base.Add(2*time.Minute)))
	require.NoError(t, err)

	ga, err := repo.GameAnalytics(ctx, "g1", 30, "")
	require.NoError(t, err)
	require.Len(t, ga.SessionCloseDaily, 1)
	assert.Equal(t, base.Format("2006-01-02"), ga.SessionCloseDaily[0].Day)
	assert.Equal(t, int64(2), ga.SessionCloseDaily[0].Matches)
}

func TestApplySessionClosed_Reprocessado_NaoDobraSerieDiaria(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	base := recentDay()
	evt := closeEvent("s1", "g1", "default", "queue_arena", "result", base, base.Add(time.Minute))
	_, err := repo.ApplySessionClosed(ctx, evt)
	require.NoError(t, err)
	_, err = repo.ApplySessionClosed(ctx, evt) // reentrega at-least-once
	require.NoError(t, err)

	ga, err := repo.GameAnalytics(ctx, "g1", 30, "")
	require.NoError(t, err)
	require.Len(t, ga.SessionCloseDaily, 1)
	assert.Equal(t, int64(1), ga.SessionCloseDaily[0].Matches, "reentrega não dobra")
}

func TestApplySessionClosed_SerieDiaria_FiltraPorModo(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	base := recentDay()
	_, err := repo.ApplySessionClosed(ctx, closeEvent("s1", "g1", "modo-a", "queue_arena", "result", base, base.Add(time.Minute)))
	require.NoError(t, err)
	_, err = repo.ApplySessionClosed(ctx, closeEvent("s2", "g1", "modo-b", "queue_arena", "result", base, base.Add(time.Minute)))
	require.NoError(t, err)

	all, err := repo.GameAnalytics(ctx, "g1", 30, "")
	require.NoError(t, err)
	require.Len(t, all.SessionCloseDaily, 1)
	assert.Equal(t, int64(2), all.SessionCloseDaily[0].Matches, "sem filtro: soma os 2 modos")

	a, err := repo.GameAnalytics(ctx, "g1", 30, "modo-a")
	require.NoError(t, err)
	require.Len(t, a.SessionCloseDaily, 1)
	assert.Equal(t, int64(1), a.SessionCloseDaily[0].Matches, "filtrado: só o modo pedido")
}
