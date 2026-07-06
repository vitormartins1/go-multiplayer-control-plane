// Package profilecache é um decorator de domain.ProfileProvider: cacheia o
// tamanho do match por gameID com um TTL curto, para não bater no game-registry
// a cada ticket (ADR-0023, spec 004). Erros não são cacheados.
package profilecache

import (
	"context"
	"sync"
	"time"

	"github.com/vitormartins1/go-game-backend/services/matchmaking-service/internal/domain"
)

var _ domain.ProfileProvider = (*CachingProvider)(nil)

type entry struct {
	size   int
	expiry time.Time
}

// CachingProvider envolve outro ProfileProvider e memoriza o tamanho por gameID.
type CachingProvider struct {
	inner domain.ProfileProvider
	ttl   time.Duration

	mu    sync.Mutex
	cache map[string]entry
}

// New cria o decorator com o provider interno e o TTL do cache.
func New(inner domain.ProfileProvider, ttl time.Duration) *CachingProvider {
	return &CachingProvider{inner: inner, ttl: ttl, cache: make(map[string]entry)}
}

// MatchSize devolve o tamanho do cache se válido; senão consulta o inner,
// cacheia (apenas em sucesso) e devolve. O lock não é mantido durante a chamada
// ao inner — dois misses concorrentes podem ambos consultar (inofensivo).
func (c *CachingProvider) MatchSize(ctx context.Context, gameID string) (int, error) {
	c.mu.Lock()
	if e, ok := c.cache[gameID]; ok && time.Now().Before(e.expiry) {
		c.mu.Unlock()
		return e.size, nil
	}
	c.mu.Unlock()

	size, err := c.inner.MatchSize(ctx, gameID)
	if err != nil {
		return 0, err // erro não é cacheado: rechama na próxima
	}

	c.mu.Lock()
	c.cache[gameID] = entry{size: size, expiry: time.Now().Add(c.ttl)}
	c.mu.Unlock()
	return size, nil
}
