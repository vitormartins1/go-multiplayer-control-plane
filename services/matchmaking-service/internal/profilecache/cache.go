// Package profilecache é um decorator de domain.ProfileProvider: cacheia a
// formação do match por (gameID, modeID) com um TTL curto, para não bater no
// game-registry a cada ticket (ADR-0023/0024). Erros não são cacheados.
package profilecache

import (
	"context"
	"sync"
	"time"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/matchmaking-service/internal/domain"
)

var _ domain.ProfileProvider = (*CachingProvider)(nil)

// key identifica uma formação no cache: o modo é dimensão junto do jogo
// (ADR-0024) — dois modos do mesmo jogo não colidem.
type key struct {
	gameID string
	modeID string
}

type entry struct {
	format domain.MatchFormat
	expiry time.Time
}

// CachingProvider envolve outro ProfileProvider e memoriza a formação por gameID.
type CachingProvider struct {
	inner domain.ProfileProvider
	ttl   time.Duration

	mu    sync.Mutex
	cache map[key]entry
}

// New cria o decorator com o provider interno e o TTL do cache.
func New(inner domain.ProfileProvider, ttl time.Duration) *CachingProvider {
	return &CachingProvider{inner: inner, ttl: ttl, cache: make(map[key]entry)}
}

// Format devolve a formação do cache se válida; senão consulta o inner, cacheia
// (apenas em sucesso) e devolve. O lock não é mantido durante a chamada ao inner
// — dois misses concorrentes podem ambos consultar (inofensivo).
func (c *CachingProvider) Format(ctx context.Context, gameID, modeID string) (domain.MatchFormat, error) {
	k := key{gameID: gameID, modeID: modeID}
	c.mu.Lock()
	if e, ok := c.cache[k]; ok && time.Now().Before(e.expiry) {
		c.mu.Unlock()
		return e.format, nil
	}
	c.mu.Unlock()

	format, err := c.inner.Format(ctx, gameID, modeID)
	if err != nil {
		return domain.MatchFormat{}, err // erro não é cacheado: rechama na próxima
	}

	c.mu.Lock()
	c.cache[k] = entry{format: format, expiry: time.Now().Add(c.ttl)}
	c.mu.Unlock()
	return format, nil
}
