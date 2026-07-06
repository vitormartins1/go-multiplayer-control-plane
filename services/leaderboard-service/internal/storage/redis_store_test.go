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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/vitormartins1/go-game-backend/services/leaderboard-service/internal/domain"
)

func newTestStore(t *testing.T) *RedisStore {
	t.Helper()
	ctx := context.Background()
	ctr, err := tcredis.Run(ctx, "redis:7")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	connStr, err := ctr.ConnectionString(ctx) // redis://host:port
	require.NoError(t, err)
	addr := strings.TrimPrefix(connStr, "redis://")
	return NewRedisStore(addr, "leaderboard:test")
}

func TestSubmitTop_OrdenadoDesc(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Submit(ctx, "g1", "score", "p1", 100))
	require.NoError(t, store.Submit(ctx, "g1", "score", "p2", 300))
	require.NoError(t, store.Submit(ctx, "g1", "score", "p3", 200))

	top, err := store.Top(ctx, "g1", "score", 10)
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
	_, score, err := store.Rank(ctx, "g1", "score", "p1")
	require.NoError(t, err)
	assert.Equal(t, int64(100), score)

	require.NoError(t, store.Submit(ctx, "g1", "score", "p1", 150)) // maior: atualiza
	_, score, err = store.Rank(ctx, "g1", "score", "p1")
	require.NoError(t, err)
	assert.Equal(t, int64(150), score)
}

func TestRank_Inexistente_ErrNotRanked(t *testing.T) {
	store := newTestStore(t)
	_, _, err := store.Rank(context.Background(), "g1", "score", "fantasma")
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
	rank0, _, err := store.Rank(ctx, "g1", "score", "p3")
	require.NoError(t, err)
	assert.Equal(t, int64(2), rank0)

	// janela [1,3] -> p4, p3, p2 com ranks 1-based 2,3,4
	win, err := store.Range(ctx, "g1", "score", 1, 3)
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
	topScore, err := store.Top(ctx, "g1", "score", 10)
	require.NoError(t, err)
	require.Len(t, topScore, 1)
	assert.Equal(t, domain.Entry{Rank: 1, PlayerID: "p1", Score: 1000}, topScore[0])

	// board kills: p2 (12) na frente do p1 (7); highscore por board.
	topKills, err := store.Top(ctx, "g1", "kills", 10)
	require.NoError(t, err)
	require.Len(t, topKills, 2)
	assert.Equal(t, "p2", topKills[0].PlayerID)
	assert.Equal(t, int64(12), topKills[0].Score)

	// board inexistente: Top vazio (não erro).
	topEmpty, err := store.Top(ctx, "g1", "assists", 10)
	require.NoError(t, err)
	assert.Empty(t, topEmpty)
}

func TestSubmit_IsoladoPorJogo(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Submit(ctx, "game-a", "score", "p1", 100))
	require.NoError(t, store.Submit(ctx, "game-b", "score", "p2", 999))

	// o ranking do game-a só tem o p1 (o p2 do game-b não vaza).
	topA, err := store.Top(ctx, "game-a", "score", 10)
	require.NoError(t, err)
	require.Len(t, topA, 1)
	assert.Equal(t, "p1", topA[0].PlayerID)

	// e o p1 não existe no ranking do game-b.
	_, _, err = store.Rank(ctx, "game-b", "score", "p1")
	assert.True(t, errors.Is(err, domain.ErrNotRanked))
}
