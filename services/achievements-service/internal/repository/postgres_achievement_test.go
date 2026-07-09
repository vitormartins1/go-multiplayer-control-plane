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

	"github.com/vitormartins1/go-multiplayer-control-plane/services/achievements-service/internal/domain"
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
			"../../migrations/004_processed_matches.sql",
			"../../migrations/005_stat_criteria.sql",
			"../../migrations/006_processed_participations.sql",
			"../../migrations/007_participation_event_id.sql",
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

// ApplyMatchResult é idempotente por matchID: reprocessar a mesma partida
// (entrega at-least-once do SQS, ADR-0020) não incrementa o progresso de novo.
func TestApplyMatchResult_Reprocessada_NaoDobraProgresso(t *testing.T) {
	repo := NewPostgresAchievementRepository(newTestPool(t))
	ctx := context.Background()
	at := time.Now().UTC()

	// p1 venceu; "first_win" (event_type=match_won, target=1) deve desbloquear.
	players := []domain.PlayerMatchInput{
		{PlayerID: "p1", EventTypes: []string{"match_played", "match_won"}},
	}

	dup, err := repo.ApplyMatchResult(ctx, sandbox, "match-1", players, at)
	require.NoError(t, err)
	assert.False(t, dup, "1ª vez: não é duplicata")

	// Reentrega da MESMA partida: no-op.
	dup, err = repo.ApplyMatchResult(ctx, sandbox, "match-1", players, at)
	require.NoError(t, err)
	assert.True(t, dup, "2ª vez: duplicata detectada")

	prog := mustProgress(t, repo, sandbox, "p1")
	assert.Equal(t, 1, prog["first_win"].Progress, "progresso não dobra com a reentrega")
	assert.False(t, prog["first_win"].UnlockedAt.IsZero(), "first_win desbloqueada")
}

// Uma NOVA partida do mesmo jogador incrementa normalmente (matchID diferente).
func TestApplyMatchResult_PartidasDistintas_Acumulam(t *testing.T) {
	repo := NewPostgresAchievementRepository(newTestPool(t))
	ctx := context.Background()
	at := time.Now().UTC()
	players := []domain.PlayerMatchInput{{PlayerID: "p1", EventTypes: []string{"match_won"}}}

	_, err := repo.ApplyMatchResult(ctx, sandbox, "match-1", players, at)
	require.NoError(t, err)
	_, err = repo.ApplyMatchResult(ctx, sandbox, "match-2", players, at)
	require.NoError(t, err)

	prog := mustProgress(t, repo, sandbox, "p1")
	assert.Equal(t, 2, prog["ten_wins"].Progress, "duas partidas distintas somam no ten_wins")
}

// Critério por stat cumulative: o valor do stat soma entre partidas e desbloqueia
// ao atingir o threshold (ADR-0023 A1). Usa o seed "sharpshooter" (kills, 100).
func TestApplyMatchResult_StatCumulative_Soma(t *testing.T) {
	repo := NewPostgresAchievementRepository(newTestPool(t))
	ctx := context.Background()
	at := time.Now().UTC()

	// duas partidas: 40 + 70 kills = 110 >= 100 (desbloqueia na 2ª, trava em 100).
	_, err := repo.ApplyMatchResult(ctx, sandbox, "m1",
		[]domain.PlayerMatchInput{{PlayerID: "p1", Stats: map[string]int64{"kills": 40}}}, at)
	require.NoError(t, err)
	prog := mustProgress(t, repo, sandbox, "p1")
	assert.Equal(t, 40, prog["sharpshooter"].Progress)
	assert.True(t, prog["sharpshooter"].UnlockedAt.IsZero(), "40 < 100: ainda bloqueada")

	_, err = repo.ApplyMatchResult(ctx, sandbox, "m2",
		[]domain.PlayerMatchInput{{PlayerID: "p1", Stats: map[string]int64{"kills": 70}}}, at)
	require.NoError(t, err)
	prog = mustProgress(t, repo, sandbox, "p1")
	assert.Equal(t, 100, prog["sharpshooter"].Progress, "acumulado trava no threshold")
	assert.False(t, prog["sharpshooter"].UnlockedAt.IsZero(), "110 >= 100: desbloqueada")
}

// Critério por stat per_match: guarda o MELHOR valor de partida e desbloqueia
// quando UMA partida atinge o threshold. Seed "ace" (kills, 5, per_match).
func TestApplyMatchResult_StatPerMatch_MelhorPartida(t *testing.T) {
	repo := NewPostgresAchievementRepository(newTestPool(t))
	ctx := context.Background()
	at := time.Now().UTC()

	// 3 kills: progride mas não desbloqueia (best=3).
	_, err := repo.ApplyMatchResult(ctx, sandbox, "m1",
		[]domain.PlayerMatchInput{{PlayerID: "p1", Stats: map[string]int64{"kills": 3}}}, at)
	require.NoError(t, err)
	prog := mustProgress(t, repo, sandbox, "p1")
	assert.Equal(t, 3, prog["ace"].Progress, "melhor partida = 3")
	assert.True(t, prog["ace"].UnlockedAt.IsZero())

	// 2 kills: pior partida NÃO regride o progresso (best continua 3).
	_, err = repo.ApplyMatchResult(ctx, sandbox, "m2",
		[]domain.PlayerMatchInput{{PlayerID: "p1", Stats: map[string]int64{"kills": 2}}}, at)
	require.NoError(t, err)
	prog = mustProgress(t, repo, sandbox, "p1")
	assert.Equal(t, 3, prog["ace"].Progress, "partida pior não regride o melhor")

	// 6 kills numa partida: >= 5 desbloqueia (progress trava em 5).
	_, err = repo.ApplyMatchResult(ctx, sandbox, "m3",
		[]domain.PlayerMatchInput{{PlayerID: "p1", Stats: map[string]int64{"kills": 6}}}, at)
	require.NoError(t, err)
	prog = mustProgress(t, repo, sandbox, "p1")
	assert.Equal(t, 5, prog["ace"].Progress, "progress trava no threshold")
	assert.False(t, prog["ace"].UnlockedAt.IsZero(), "6 >= 5 numa partida: desbloqueada")
}

// Reprocessar a mesma partida não dobra o acumulado do stat (idempotência por
// matchID — a tx que marca processed_matches é a mesma que soma).
func TestApplyMatchResult_StatCumulative_Idempotente(t *testing.T) {
	repo := NewPostgresAchievementRepository(newTestPool(t))
	ctx := context.Background()
	at := time.Now().UTC()
	players := []domain.PlayerMatchInput{{PlayerID: "p1", Stats: map[string]int64{"kills": 30}}}

	_, err := repo.ApplyMatchResult(ctx, sandbox, "m1", players, at)
	require.NoError(t, err)
	dup, err := repo.ApplyMatchResult(ctx, sandbox, "m1", players, at)
	require.NoError(t, err)
	assert.True(t, dup, "reentrega detectada")

	prog := mustProgress(t, repo, sandbox, "p1")
	assert.Equal(t, 30, prog["sharpshooter"].Progress, "acumulado não dobra na reentrega")
}

// Stat ausente do resultado -> a conquista daquele stat não ganha progresso.
func TestApplyMatchResult_StatAusente_SemProgresso(t *testing.T) {
	repo := NewPostgresAchievementRepository(newTestPool(t))
	ctx := context.Background()
	at := time.Now().UTC()

	// só "assists" no resultado; nenhuma conquista casa "assists" no seed.
	_, err := repo.ApplyMatchResult(ctx, sandbox, "m1",
		[]domain.PlayerMatchInput{{PlayerID: "p1", Stats: map[string]int64{"assists": 9}}}, at)
	require.NoError(t, err)

	prog := mustProgress(t, repo, sandbox, "p1")
	_, hasSharp := prog["sharpshooter"]
	assert.False(t, hasSharp, "kills ausente -> sharpshooter sem progresso")
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

// ApplyParticipationEvent é idempotente por (session_id, player_id, event):
// reprocessar o mesmo evento não incrementa o progresso de novo (spec 009).
func TestApplyParticipationEvent_Reprocessado_NaoDobra(t *testing.T) {
	repo := NewPostgresAchievementRepository(newTestPool(t))
	ctx := context.Background()
	at := time.Now().UTC()
	require.NoError(t, repo.CreateAchievement(ctx, &domain.Achievement{
		GameID: sandbox, ID: "first_invasion", Name: "Primeira invasão", Description: "d",
		EventType: domain.EventInvasionWon, Target: 1,
	}))

	types := domain.ParticipationEventTypes("invader", "HOST_KILLED", "SUCCESS")
	dup, err := repo.ApplyParticipationEvent(ctx, sandbox, "world-1", "inv-1", "HOST_KILLED", "", types, at)
	require.NoError(t, err)
	assert.False(t, dup)

	dup, err = repo.ApplyParticipationEvent(ctx, sandbox, "world-1", "inv-1", "HOST_KILLED", "", types, at)
	require.NoError(t, err)
	assert.True(t, dup, "reentrega do mesmo evento (sem eventId) é no-op via fallback determinístico")

	prog := mustProgress(t, repo, sandbox, "inv-1")
	assert.Equal(t, 1, prog["first_invasion"].Progress, "não dobra na reentrega")
	assert.False(t, prog["first_invasion"].UnlockedAt.IsZero(), "desbloqueada")
}

// Dois eventId diferentes (mesma sessão/jogador/evento) contam duas vezes —
// o cenário que o follow-up #4 destrava (ex.: boss que respawna).
func TestApplyParticipationEvent_EventIDsDiferentes_ContaDuasVezes(t *testing.T) {
	repo := NewPostgresAchievementRepository(newTestPool(t))
	ctx := context.Background()
	at := time.Now().UTC()
	require.NoError(t, repo.CreateAchievement(ctx, &domain.Achievement{
		GameID: sandbox, ID: "boss_slayer", Name: "Caçador de chefes", Description: "d",
		EventType: domain.EventCoopBossHelped, Target: 5,
	}))
	types := domain.ParticipationEventTypes("coop", "BOSS_DEFEATED", "SUCCESS")

	_, err := repo.ApplyParticipationEvent(ctx, sandbox, "world-1", "coop-1", "BOSS_DEFEATED", "evt-a", types, at)
	require.NoError(t, err)
	_, err = repo.ApplyParticipationEvent(ctx, sandbox, "world-1", "coop-1", "BOSS_DEFEATED", "evt-b", types, at)
	require.NoError(t, err)

	prog := mustProgress(t, repo, sandbox, "coop-1")
	assert.Equal(t, 2, prog["boss_slayer"].Progress, "mesma sessão, eventId diferente -> conta as duas ocorrências")
}

// Reenviar o MESMO eventId (retry) continua idempotente, mesmo comportamento
// de antes só que pela chave nova.
func TestApplyParticipationEvent_MesmoEventID_NaoDobra(t *testing.T) {
	repo := NewPostgresAchievementRepository(newTestPool(t))
	ctx := context.Background()
	at := time.Now().UTC()
	require.NoError(t, repo.CreateAchievement(ctx, &domain.Achievement{
		GameID: sandbox, ID: "boss_slayer2", Name: "Caçador de chefes 2", Description: "d",
		EventType: domain.EventCoopBossHelped, Target: 5,
	}))
	types := domain.ParticipationEventTypes("coop", "BOSS_DEFEATED", "SUCCESS")

	dup1, err := repo.ApplyParticipationEvent(ctx, sandbox, "world-1", "coop-1", "BOSS_DEFEATED", "evt-retry", types, at)
	require.NoError(t, err)
	assert.False(t, dup1)
	dup2, err := repo.ApplyParticipationEvent(ctx, sandbox, "world-1", "coop-1", "BOSS_DEFEATED", "evt-retry", types, at)
	require.NoError(t, err)
	assert.True(t, dup2, "retry do mesmo eventId é no-op")

	prog := mustProgress(t, repo, sandbox, "coop-1")
	assert.Equal(t, 1, prog["boss_slayer2"].Progress)
}

// Sessões distintas do mesmo jogador acumulam (a chave inclui session_id).
func TestApplyParticipationEvent_SessoesDistintas_Acumulam(t *testing.T) {
	repo := NewPostgresAchievementRepository(newTestPool(t))
	ctx := context.Background()
	at := time.Now().UTC()
	require.NoError(t, repo.CreateAchievement(ctx, &domain.Achievement{
		GameID: sandbox, ID: "social_butterfly", Name: "Sociável", Description: "d",
		EventType: domain.EventWorldJoined, Target: 5,
	}))
	types := domain.ParticipationEventTypes("coop", "JOINED", "NEUTRAL")

	_, err := repo.ApplyParticipationEvent(ctx, sandbox, "world-1", "coop-1", "JOINED", "", types, at)
	require.NoError(t, err)
	_, err = repo.ApplyParticipationEvent(ctx, sandbox, "world-2", "coop-1", "JOINED", "", types, at)
	require.NoError(t, err)

	prog := mustProgress(t, repo, sandbox, "coop-1")
	assert.Equal(t, 2, prog["social_butterfly"].Progress, "duas sessões somam")
}

// Evento sem event type (ex.: LEFT) é marcado processado sem tocar progresso.
func TestApplyParticipationEvent_SemEventType_ProcessaSemProgresso(t *testing.T) {
	repo := NewPostgresAchievementRepository(newTestPool(t))
	ctx := context.Background()
	at := time.Now().UTC()

	dup, err := repo.ApplyParticipationEvent(ctx, sandbox, "world-9", "p-9", "LEFT", "", nil, at)
	require.NoError(t, err)
	assert.False(t, dup)
	// reprocessar continua no-op idempotente.
	dup, err = repo.ApplyParticipationEvent(ctx, sandbox, "world-9", "p-9", "LEFT", "", nil, at)
	require.NoError(t, err)
	assert.True(t, dup)

	assert.Empty(t, mustProgress(t, repo, sandbox, "p-9"), "sem progresso")
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
