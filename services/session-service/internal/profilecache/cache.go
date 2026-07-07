// Package profilecache é um decorator de domain.ProfileProvider: cacheia o
// snapshot do perfil por (gameID, modeID) com um TTL curto, para o Create não
// bater no game-registry a cada partida (ADR-0024, spec 017). Erros não são
// cacheados. Mesmo padrão do matchmaking (GO-LEARNINGS #21).
package profilecache

import (
	"context"
	"sync"
	"time"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/domain"
)

var _ domain.ProfileProvider = (*CachingProvider)(nil)

// key identifica um perfil no cache: o modo é dimensão junto do jogo (ADR-0024).
type key struct {
	gameID string
	modeID string
}

type entry struct {
	snap   domain.ProfileSnapshot
	expiry time.Time
}

// CachingProvider envolve outro ProfileProvider e memoriza o snapshot por
// (gameID, modeID).
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

// ModeProfile devolve o snapshot do cache se válido; senão consulta o inner,
// cacheia (apenas em sucesso) e devolve. O lock não é mantido durante a chamada
// ao inner — dois misses concorrentes podem ambos consultar (inofensivo).
func (c *CachingProvider) ModeProfile(ctx context.Context, gameID, modeID string) (domain.ProfileSnapshot, error) {
	k := key{gameID: gameID, modeID: modeID}
	c.mu.Lock()
	if e, ok := c.cache[k]; ok && time.Now().Before(e.expiry) {
		c.mu.Unlock()
		return e.snap, nil
	}
	c.mu.Unlock()

	snap, err := c.inner.ModeProfile(ctx, gameID, modeID)
	if err != nil {
		return domain.ProfileSnapshot{}, err // erro não é cacheado: rechama na próxima
	}

	c.mu.Lock()
	c.cache[k] = entry{snap: snap, expiry: time.Now().Add(c.ttl)}
	c.mu.Unlock()
	return snap, nil
}
