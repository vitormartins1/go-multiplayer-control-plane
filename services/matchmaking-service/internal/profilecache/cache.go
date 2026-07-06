// Package profilecache é um decorator de domain.ProfileProvider: cacheia a
// formação do match por gameID com um TTL curto, para não bater no game-registry
// a cada ticket (ADR-0023, specs 004/005). Erros não são cacheados.
package profilecache

import (
	"context"
	"sync"
	"time"

	"github.com/vitormartins1/go-game-backend/services/matchmaking-service/internal/domain"
)

var _ domain.ProfileProvider = (*CachingProvider)(nil)

type entry struct {
	format domain.MatchFormat
	expiry time.Time
}

// CachingProvider envolve outro ProfileProvider e memoriza a formação por gameID.
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

// Format devolve a formação do cache se válida; senão consulta o inner, cacheia
// (apenas em sucesso) e devolve. O lock não é mantido durante a chamada ao inner
// — dois misses concorrentes podem ambos consultar (inofensivo).
func (c *CachingProvider) Format(ctx context.Context, gameID string) (domain.MatchFormat, error) {
	c.mu.Lock()
	if e, ok := c.cache[gameID]; ok && time.Now().Before(e.expiry) {
		c.mu.Unlock()
		return e.format, nil
	}
	c.mu.Unlock()

	format, err := c.inner.Format(ctx, gameID)
	if err != nil {
		return domain.MatchFormat{}, err // erro não é cacheado: rechama na próxima
	}

	c.mu.Lock()
	c.cache[gameID] = entry{format: format, expiry: time.Now().Add(c.ttl)}
	c.mu.Unlock()
	return format, nil
}
