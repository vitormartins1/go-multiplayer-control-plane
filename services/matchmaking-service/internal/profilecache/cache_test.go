package profilecache

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-game-backend/services/matchmaking-service/internal/domain"
)

// countingProvider conta as chamadas ao Format (o "inner" do decorator).
type countingProvider struct {
	mu    sync.Mutex
	size  int
	err   error
	calls int
}

func (c *countingProvider) Format(_ context.Context, _ string) (domain.MatchFormat, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	return domain.MatchFormat{Size: c.size, Structure: domain.StructureFFA}, c.err
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
		format, err := c.Format(context.Background(), "g1")
		require.NoError(t, err)
		assert.Equal(t, 4, format.Size)
	}
	assert.Equal(t, 1, inner.count(), "hits no cache não chamam o inner")
}

func TestMatchSize_Expirado_Rechama(t *testing.T) {
	inner := &countingProvider{size: 2}
	c := New(inner, 0) // TTL 0: toda leitura já está expirada

	_, _ = c.Format(context.Background(), "g1")
	_, _ = c.Format(context.Background(), "g1")
	assert.Equal(t, 2, inner.count(), "expirado -> rechama")
}

func TestMatchSize_ErroNaoCacheado(t *testing.T) {
	inner := &countingProvider{err: errors.New("registry fora")}
	c := New(inner, time.Hour)

	_, err1 := c.Format(context.Background(), "g1")
	_, err2 := c.Format(context.Background(), "g1")
	require.Error(t, err1)
	require.Error(t, err2)
	assert.Equal(t, 2, inner.count(), "erro não é cacheado -> rechama")
}

func TestMatchSize_JogosDistintos_UmaChamadaCada(t *testing.T) {
	inner := &countingProvider{size: 2}
	c := New(inner, time.Hour)

	_, _ = c.Format(context.Background(), "g1")
	_, _ = c.Format(context.Background(), "g2")
	_, _ = c.Format(context.Background(), "g1") // hit
	assert.Equal(t, 2, inner.count(), "uma chamada por jogo distinto")
}
