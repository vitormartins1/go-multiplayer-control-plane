// Package allocator implementa o pool de game servers. No MVP é um pool fixo em
// memória (mock); o adapter real (Agones Allocation API) entra no deploy
// (ADR-0011/0012) e só troca a implementação atrás da Port domain.ServerAllocator.
package allocator

import (
	"context"
	"fmt"
	"sync"

	"github.com/c130n/game-backend/services/session-service/internal/domain"
)

var _ domain.ServerAllocator = (*Static)(nil)

// slot é uma vaga do pool: um server e o estado de alocação.
type slot struct {
	server    domain.Server
	allocated bool
}

// Static é um pool fixo de game servers em memória. Allocate/Release são
// atômicos sob mutex (GO-LEARNINGS #01): cada server vai para no máximo uma
// sessão por vez.
type Static struct {
	mu    sync.Mutex
	slots []*slot
}

// New cria o pool a partir dos endereços informados. Cada endereço vira um
// game server livre com ID estável (server-0, server-1, ...).
func New(addresses []string) *Static {
	slots := make([]*slot, len(addresses))
	for i, addr := range addresses {
		slots[i] = &slot{server: domain.Server{ID: fmt.Sprintf("server-%d", i), Address: addr}}
	}
	return &Static{slots: slots}
}

// Allocate reserva o primeiro server livre. ErrNoServer se o pool esgotar.
func (s *Static) Allocate(_ context.Context) (domain.Server, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sl := range s.slots {
		if !sl.allocated {
			sl.allocated = true
			return sl.server, nil
		}
	}
	return domain.Server{}, domain.ErrNoServer
}

// Release devolve o server ao pool. Liberar um server já livre (ou inexistente)
// é no-op idempotente.
func (s *Static) Release(_ context.Context, serverID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sl := range s.slots {
		if sl.server.ID == serverID {
			sl.allocated = false
			return nil
		}
	}
	return nil
}
