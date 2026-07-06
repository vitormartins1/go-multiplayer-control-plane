package profilecache

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countingProvider conta as chamadas ao MatchSize (o "inner" do decorator).
type countingProvider struct {
	mu    sync.Mutex
	size  int
	err   error
	calls int
}

func (c *countingProvider) MatchSize(_ context.Context, _ string) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	return c.size, c.err
}

func (c *countingProvider) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

func TestMatchSize_DentroDoTTL_UmaChamada(t *testing.T) {
	inner := &countingProvider{size: 4}
	c := New(inner, time.Hour)

	for i := 0; i < 3; i++ {
		size, err := c.MatchSize(context.Background(), "g1")
		require.NoError(t, err)
		assert.Equal(t, 4, size)
	}
	assert.Equal(t, 1, inner.count(), "hits no cache não chamam o inner")
}

func TestMatchSize_Expirado_Rechama(t *testing.T) {
	inner := &countingProvider{size: 2}
	c := New(inner, 0) // TTL 0: toda leitura já está expirada

	_, _ = c.MatchSize(context.Background(), "g1")
	_, _ = c.MatchSize(context.Background(), "g1")
	assert.Equal(t, 2, inner.count(), "expirado -> rechama")
}

func TestMatchSize_ErroNaoCacheado(t *testing.T) {
	inner := &countingProvider{err: errors.New("registry fora")}
	c := New(inner, time.Hour)

	_, err1 := c.MatchSize(context.Background(), "g1")
	_, err2 := c.MatchSize(context.Background(), "g1")
	require.Error(t, err1)
	require.Error(t, err2)
	assert.Equal(t, 2, inner.count(), "erro não é cacheado -> rechama")
}

func TestMatchSize_JogosDistintos_UmaChamadaCada(t *testing.T) {
	inner := &countingProvider{size: 2}
	c := New(inner, time.Hour)

	_, _ = c.MatchSize(context.Background(), "g1")
	_, _ = c.MatchSize(context.Background(), "g2")
	_, _ = c.MatchSize(context.Background(), "g1") // hit
	assert.Equal(t, 2, inner.count(), "uma chamada por jogo distinto")
}
