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

	"github.com/c130n/game-backend/services/leaderboard-service/internal/domain"
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

	require.NoError(t, store.Submit(ctx, "p1", 100))
	require.NoError(t, store.Submit(ctx, "p2", 300))
	require.NoError(t, store.Submit(ctx, "p3", 200))

	top, err := store.Top(ctx, 10)
	require.NoError(t, err)
	require.Len(t, top, 3)
	assert.Equal(t, domain.Entry{Rank: 1, PlayerID: "p2", Score: 300}, top[0])
	assert.Equal(t, domain.Entry{Rank: 2, PlayerID: "p3", Score: 200}, top[1])
	assert.Equal(t, domain.Entry{Rank: 3, PlayerID: "p1", Score: 100}, top[2])
}

func TestSubmit_Highscore_NaoRebaixa(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Submit(ctx, "p1", 100))
	require.NoError(t, store.Submit(ctx, "p1", 50)) // menor: não rebaixa
	_, score, err := store.Rank(ctx, "p1")
	require.NoError(t, err)
	assert.Equal(t, int64(100), score)

	require.NoError(t, store.Submit(ctx, "p1", 150)) // maior: atualiza
	_, score, err = store.Rank(ctx, "p1")
	require.NoError(t, err)
	assert.Equal(t, int64(150), score)
}

func TestRank_Inexistente_ErrNotRanked(t *testing.T) {
	store := newTestStore(t)
	_, _, err := store.Rank(context.Background(), "fantasma")
	assert.True(t, errors.Is(err, domain.ErrNotRanked))
}

func TestRange_JanelaAoRedor(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	// 5 jogadores: scores 500..100 -> ranks 0..4
	for i, p := range []string{"p5", "p4", "p3", "p2", "p1"} {
		require.NoError(t, store.Submit(ctx, p, int64((5-i)*100)))
	}
	// posição 0-based do p3 deve ser 2
	rank0, _, err := store.Rank(ctx, "p3")
	require.NoError(t, err)
	assert.Equal(t, int64(2), rank0)

	// janela [1,3] -> p4, p3, p2 com ranks 1-based 2,3,4
	win, err := store.Range(ctx, 1, 3)
	require.NoError(t, err)
	require.Len(t, win, 3)
	assert.Equal(t, "p4", win[0].PlayerID)
	assert.Equal(t, int64(2), win[0].Rank)
	assert.Equal(t, "p3", win[1].PlayerID)
	assert.Equal(t, int64(3), win[1].Rank)
}
