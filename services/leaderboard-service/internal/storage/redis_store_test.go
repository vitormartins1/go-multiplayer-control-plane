//go:build integration

// Testes de integração com Redis real (testcontainers). Rodar com:
//
//	go test -tags=integration ./services/leaderboard-service/internal/storage/
//
// Requer Docker em execução.
package storage

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/leaderboard-service/internal/domain"
)

const alltime = domain.WindowAllTime

// fixedNow é um instante estável para os períodos serem determinísticos
// (2026-07-08 é uma quarta-feira, semana ISO 2026W28).
var fixedNow = time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)

func newTestStore(t *testing.T) *RedisStore {
	t.Helper()
	ctx := context.Background()
	ctr, err := tcredis.Run(ctx, "redis:7")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	connStr, err := ctr.ConnectionString(ctx) // redis://host:port
	require.NoError(t, err)
	addr := strings.TrimPrefix(connStr, "redis://")
	store := NewRedisStore(addr, "leaderboard:test")
	store.now = func() time.Time { return fixedNow } // relógio fixado (spec 007)
	return store
}

func TestSubmitTop_OrdenadoDesc(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Submit(ctx, "g1", "score", "p1", 100))
	require.NoError(t, store.Submit(ctx, "g1", "score", "p2", 300))
	require.NoError(t, store.Submit(ctx, "g1", "score", "p3", 200))

	top, err := store.Top(ctx, "g1", "score", alltime, 10)
	require.NoError(t, err)
	require.Len(t, top, 3)
	assert.Equal(t, domain.Entry{Rank: 1, PlayerID: "p2", Score: 300}, top[0])
	assert.Equal(t, domain.Entry{Rank: 2, PlayerID: "p3", Score: 200}, top[1])
	assert.Equal(t, domain.Entry{Rank: 3, PlayerID: "p1", Score: 100}, top[2])
}

func TestSubmit_Highscore_NaoRebaixa(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Submit(ctx, "g1", "score", "p1", 100))
	require.NoError(t, store.Submit(ctx, "g1", "score", "p1", 50)) // menor: não rebaixa
	_, score, err := store.Rank(ctx, "g1", "score", alltime, "p1")
	require.NoError(t, err)
	assert.Equal(t, int64(100), score)

	require.NoError(t, store.Submit(ctx, "g1", "score", "p1", 150)) // maior: atualiza
	_, score, err = store.Rank(ctx, "g1", "score", alltime, "p1")
	require.NoError(t, err)
	assert.Equal(t, int64(150), score)
}

func TestRank_Inexistente_ErrNotRanked(t *testing.T) {
	store := newTestStore(t)
	_, _, err := store.Rank(context.Background(), "g1", "score", alltime, "fantasma")
	assert.True(t, errors.Is(err, domain.ErrNotRanked))
}

func TestRange_JanelaAoRedor(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	// 5 jogadores: scores 500..100 -> ranks 0..4
	for i, p := range []string{"p5", "p4", "p3", "p2", "p1"} {
		require.NoError(t, store.Submit(ctx, "g1", "score", p, int64((5-i)*100)))
	}
	// posição 0-based do p3 deve ser 2
	rank0, _, err := store.Rank(ctx, "g1", "score", alltime, "p3")
	require.NoError(t, err)
	assert.Equal(t, int64(2), rank0)

	// janela [1,3] -> p4, p3, p2 com ranks 1-based 2,3,4 (paginação: ranks contínuos)
	win, err := store.Range(ctx, "g1", "score", alltime, 1, 3)
	require.NoError(t, err)
	require.Len(t, win, 3)
	assert.Equal(t, "p4", win[0].PlayerID)
	assert.Equal(t, int64(2), win[0].Rank)
	assert.Equal(t, "p3", win[1].PlayerID)
	assert.Equal(t, int64(3), win[1].Rank)
}

func TestSubmit_BoardsIndependentes(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// mesmo jogo, boards distintos: score e kills não se misturam.
	require.NoError(t, store.Submit(ctx, "g1", "score", "p1", 1000))
	require.NoError(t, store.Submit(ctx, "g1", "kills", "p1", 7))
	require.NoError(t, store.Submit(ctx, "g1", "kills", "p2", 12))

	// board score: só o p1, com 1000.
	topScore, err := store.Top(ctx, "g1", "score", alltime, 10)
	require.NoError(t, err)
	require.Len(t, topScore, 1)
	assert.Equal(t, domain.Entry{Rank: 1, PlayerID: "p1", Score: 1000}, topScore[0])

	// board kills: p2 (12) na frente do p1 (7); highscore por board.
	topKills, err := store.Top(ctx, "g1", "kills", alltime, 10)
	require.NoError(t, err)
	require.Len(t, topKills, 2)
	assert.Equal(t, "p2", topKills[0].PlayerID)
	assert.Equal(t, int64(12), topKills[0].Score)

	// board inexistente: Top vazio (não erro).
	topEmpty, err := store.Top(ctx, "g1", "assists", alltime, 10)
	require.NoError(t, err)
	assert.Empty(t, topEmpty)
}

func TestSubmit_IsoladoPorJogo(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Submit(ctx, "game-a", "score", "p1", 100))
	require.NoError(t, store.Submit(ctx, "game-b", "score", "p2", 999))

	// o ranking do game-a só tem o p1 (o p2 do game-b não vaza).
	topA, err := store.Top(ctx, "game-a", "score", alltime, 10)
	require.NoError(t, err)
	require.Len(t, topA, 1)
	assert.Equal(t, "p1", topA[0].PlayerID)

	// e o p1 não existe no ranking do game-b.
	_, _, err = store.Rank(ctx, "game-b", "score", alltime, "p1")
	assert.True(t, errors.Is(err, domain.ErrNotRanked))
}

// ApplyCumulative soma (ZINCRBY) entre partidas distintas e é idempotente por
// matchID (ADR-0023 A2): reprocessar a mesma partida não dobra o total.
func TestApplyCumulative_SomaEntrePartidasEIdempotente(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	m1 := []domain.Increment{
		{Board: domain.BoardWins, PlayerID: "p1", Delta: 1},
		{Board: domain.TotalBoard("kills"), PlayerID: "p1", Delta: 5},
	}
	m2 := []domain.Increment{
		{Board: domain.BoardWins, PlayerID: "p1", Delta: 1},
		{Board: domain.TotalBoard("kills"), PlayerID: "p1", Delta: 8},
	}

	dup, err := store.ApplyCumulative(ctx, "g1", "match-1", m1)
	require.NoError(t, err)
	assert.False(t, dup, "1ª aplicação não é dup")

	// reentrega da MESMA partida: no-op (não dobra).
	dup, err = store.ApplyCumulative(ctx, "g1", "match-1", m1)
	require.NoError(t, err)
	assert.True(t, dup, "reentrega detectada")

	// nova partida acumula.
	dup, err = store.ApplyCumulative(ctx, "g1", "match-2", m2)
	require.NoError(t, err)
	assert.False(t, dup)

	// wins: 1 (m1) + 1 (m2) = 2; a reentrega de m1 não contou.
	_, wins, err := store.Rank(ctx, "g1", domain.BoardWins, alltime, "p1")
	require.NoError(t, err)
	assert.Equal(t, int64(2), wins)

	// kills_total: 5 + 8 = 13.
	_, kills, err := store.Rank(ctx, "g1", domain.TotalBoard("kills"), alltime, "p1")
	require.NoError(t, err)
	assert.Equal(t, int64(13), kills)
}

// Boards cumulativos e highscore convivem sem se misturar no mesmo jogo.
func TestApplyCumulative_NaoAfetaBoardsHighscore(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Submit(ctx, "g1", "kills", "p1", 7)) // highscore "kills" (melhor de uma partida)
	_, err := store.ApplyCumulative(ctx, "g1", "m1",
		[]domain.Increment{{Board: domain.TotalBoard("kills"), PlayerID: "p1", Delta: 7}})
	require.NoError(t, err)

	_, best, err := store.Rank(ctx, "g1", "kills", alltime, "p1")
	require.NoError(t, err)
	assert.Equal(t, int64(7), best, "board highscore intacto")

	_, total, err := store.Rank(ctx, "g1", domain.TotalBoard("kills"), alltime, "p1")
	require.NoError(t, err)
	assert.Equal(t, int64(7), total, "board cumulativo é independente")
}

// --- janelas de tempo (spec 007) ---

// Um write espalha nas 4 janelas: o score aparece em todas.
func TestSubmit_EspalhaNas4Janelas(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Submit(ctx, "g1", "score", "p1", 100))

	for _, w := range domain.Windows {
		_, score, err := store.Rank(ctx, "g1", "score", w, "p1")
		require.NoError(t, err, "janela %s", w)
		assert.Equal(t, int64(100), score, "janela %s", w)
	}
}

// As keys periódicas ganham TTL; a all-time não (spec 007: TTL = só limpeza).
func TestSubmit_TTLNasPeriodicasENaoNaAllTime(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Submit(ctx, "g1", "score", "p1", 100))

	ttlAll := store.client.TTL(ctx, store.windowKey("g1", "score", alltime, fixedNow)).Val()
	assert.Equal(t, time.Duration(-1), ttlAll, "all-time sem TTL (-1 = sem expire)")

	for _, w := range []string{domain.WindowDaily, domain.WindowWeekly, domain.WindowMonthly} {
		ttl := store.client.TTL(ctx, store.windowKey("g1", "score", w, fixedNow)).Val()
		assert.Greater(t, ttl, time.Duration(0), "janela %s com TTL", w)
	}
}

// O cumulativo também espalha nas 4 janelas, e a reentrega é no-op em TODAS
// (guarda de idempotência única).
func TestApplyCumulative_EspalhaEDupENoOpEmTodasAsJanelas(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	incrs := []domain.Increment{{Board: domain.BoardWins, PlayerID: "p1", Delta: 1}}
	_, err := store.ApplyCumulative(ctx, "g1", "m1", incrs)
	require.NoError(t, err)

	dup, err := store.ApplyCumulative(ctx, "g1", "m1", incrs) // reentrega
	require.NoError(t, err)
	assert.True(t, dup)

	for _, w := range domain.Windows {
		_, wins, err := store.Rank(ctx, "g1", domain.BoardWins, w, "p1")
		require.NoError(t, err, "janela %s", w)
		assert.Equal(t, int64(1), wins, "janela %s: reentrega não somou", w)
	}
}

// Virada de período: avançar o relógio muda a key diária — o ranking do dia
// novo nasce vazio e o do dia anterior fica preservado (até o TTL).
func TestJanelas_ViradaDePeriodo_DiaNovoVazio(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Submit(ctx, "g1", "score", "p1", 100))

	// avança para o dia seguinte (mesma semana ISO e mesmo mês).
	store.now = func() time.Time { return fixedNow.Add(24 * time.Hour) }

	_, _, err := store.Rank(ctx, "g1", "score", domain.WindowDaily, "p1")
	assert.True(t, errors.Is(err, domain.ErrNotRanked), "dia novo: ranking diário vazio")

	// semana/mês/all-time seguem valendo.
	for _, w := range []string{domain.WindowWeekly, domain.WindowMonthly, alltime} {
		_, score, err := store.Rank(ctx, "g1", "score", w, "p1")
		require.NoError(t, err, "janela %s", w)
		assert.Equal(t, int64(100), score, "janela %s", w)
	}

	// e a key do dia anterior continua lá (retida pelo TTL).
	prev, err := store.client.Exists(ctx, store.windowKey("g1", "score", domain.WindowDaily, fixedNow)).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), prev, "key do dia anterior preservada")
}

// ForceScore rebaixa em todas as janelas (admin override, ZADD sem GT).
func TestForceScore_SubstituiEmTodasAsJanelas(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Submit(ctx, "g1", "score", "p1", 100))
	require.NoError(t, store.ForceScore(ctx, "g1", "score", "p1", 10)) // rebaixa

	for _, w := range domain.Windows {
		_, score, err := store.Rank(ctx, "g1", "score", w, "p1")
		require.NoError(t, err, "janela %s", w)
		assert.Equal(t, int64(10), score, "janela %s", w)
	}
}

// Spec 008: a guarda de idempotência é uma key por partida com TTL (o SET
// único crescia para sempre).
func TestApplyCumulative_GuardaPorPartidaComTTL(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.ApplyCumulative(ctx, "g1", "match-1", []domain.Increment{{Board: domain.BoardWins, PlayerID: "p1", Delta: 1}})
	require.NoError(t, err)

	ttl, err := store.client.TTL(ctx, store.processedKey("g1", "match-1")).Result()
	require.NoError(t, err)
	assert.Greater(t, ttl, time.Duration(0), "a guarda expira")
	assert.LessOrEqual(t, ttl, processedTTL)

	exists, err := store.client.Exists(ctx, store.legacyProcessedKey("g1")).Result()
	require.NoError(t, err)
	assert.Zero(t, exists, "o SET legado não é mais escrito")
}

// Spec 008 (compat): partida aplicada antes do deploy (só no SET legado)
// continua sendo detectada como reentrega.
func TestApplyCumulative_PartidaNoSetLegado_Dup(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	require.NoError(t, store.client.SAdd(ctx, store.legacyProcessedKey("g1"), "match-antiga").Err())

	dup, err := store.ApplyCumulative(ctx, "g1", "match-antiga", []domain.Increment{{Board: domain.BoardWins, PlayerID: "p1", Delta: 1}})
	require.NoError(t, err)
	assert.True(t, dup)

	_, _, err = store.Rank(ctx, "g1", domain.BoardWins, alltime, "p1")
	assert.ErrorIs(t, err, domain.ErrNotRanked, "reentrega do legado não soma")
}
