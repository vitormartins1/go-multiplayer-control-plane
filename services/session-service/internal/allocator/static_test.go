package allocator

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/domain"
)

func TestAllocate_ServersDistintos_AteEsgotar(t *testing.T) {
	pool := New([]string{"10.0.0.1:7777", "10.0.0.2:7777"})
	ctx := context.Background()

	s1, err := pool.Allocate(ctx, "match-1", "game-1")
	require.NoError(t, err)
	s2, err := pool.Allocate(ctx, "match-1", "game-1")
	require.NoError(t, err)
	assert.NotEqual(t, s1.ID, s2.ID, "dois Allocate não recebem o mesmo server")

	_, err = pool.Allocate(ctx, "match-1", "game-1")
	assert.ErrorIs(t, err, domain.ErrNoServer)
}

func TestRelease_ReabreVaga(t *testing.T) {
	pool := New([]string{"10.0.0.1:7777"})
	ctx := context.Background()

	s1, err := pool.Allocate(ctx, "match-1", "game-1")
	require.NoError(t, err)
	_, err = pool.Allocate(ctx, "match-1", "game-1")
	require.ErrorIs(t, err, domain.ErrNoServer)

	require.NoError(t, pool.Release(ctx, s1.ID))

	s2, err := pool.Allocate(ctx, "match-1", "game-1")
	require.NoError(t, err)
	assert.Equal(t, s1.ID, s2.ID, "o server liberado volta a ser alocável")
}

// Com -race: muitos Allocate concorrentes nunca entregam o mesmo server duas
// vezes (alocação atômica — GO-LEARNINGS #01).
func TestAllocate_Concorrente_SemDuplicar(t *testing.T) {
	const n = 50
	pool := New(addrs(n))

	var mu sync.Mutex
	seen := map[string]bool{}
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := pool.Allocate(context.Background(), "match-1", "game-1")
			require.NoError(t, err)
			mu.Lock()
			assert.False(t, seen[s.ID], "server %s alocado duas vezes", s.ID)
			seen[s.ID] = true
			mu.Unlock()
		}()
	}
	wg.Wait()
	assert.Len(t, seen, n)
}

func addrs(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = "10.0.0.1:7777"
	}
	return out
}
