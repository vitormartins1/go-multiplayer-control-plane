// Package service contém o matchmaker: a fila concorrente que forma partidas.
// Só depende das interfaces de domain (ADR-0002). É o "core gordo de
// concorrência" — ver GO-LEARNINGS #01.
package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/c130n/game-backend/services/matchmaking-service/internal/domain"
)

var _ domain.Matchmaker = (*Matchmaker)(nil)

type ticket struct {
	playerID string
	ch       chan domain.Match // buffer 1: tryForm nunca bloqueia
}

// Matchmaker mantém a fila de tickets em memória e forma matches FIFO.
type Matchmaker struct {
	matchSize int
	interval  time.Duration

	mu            sync.Mutex
	queue         []*ticket
	matchesFormed int64
}

// New cria o matchmaker (tamanho do match e intervalo do loop de formação).
func New(matchSize int, interval time.Duration) *Matchmaker {
	return &Matchmaker{matchSize: matchSize, interval: interval}
}

// Join enfileira o jogador (um ticket por jogador) e devolve o canal do Match.
// Quando o ctx é cancelado (stream fechado), o ticket sai da fila.
func (m *Matchmaker) Join(ctx context.Context, playerID string) (<-chan domain.Match, error) {
	t := &ticket{playerID: playerID, ch: make(chan domain.Match, 1)}

	m.mu.Lock()
	m.removeByPlayerLocked(playerID) // garante um ticket por jogador
	m.queue = append(m.queue, t)
	m.mu.Unlock()

	go func() {
		<-ctx.Done()
		m.remove(t)
	}()
	return t.ch, nil
}

// Run roda o loop de formação até o ctx ser cancelado.
func (m *Matchmaker) Run(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.tryForm()
		}
	}
}

// tryForm agrupa matchSize tickets (FIFO) em matches, de forma atômica.
func (m *Matchmaker) tryForm() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for len(m.queue) >= m.matchSize {
		group := m.queue[:m.matchSize:m.matchSize]
		m.queue = m.queue[m.matchSize:]

		match := domain.Match{ID: uuid.NewString(), Players: playerIDs(group)}
		match.ConnectInfo = fmt.Sprintf("mock://session/%s", match.ID) // mock até o session existir
		for _, t := range group {
			t.ch <- match // buffered: não bloqueia mesmo se o cliente já saiu
			close(t.ch)
		}
		m.matchesFormed++
	}
}

// QueueSize devolve o tamanho atual da fila (métrica).
func (m *Matchmaker) QueueSize() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.queue)
}

// MatchesTotal devolve o total de matches formados (métrica).
func (m *Matchmaker) MatchesTotal() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return float64(m.matchesFormed)
}

func (m *Matchmaker) remove(t *ticket) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, q := range m.queue {
		if q == t {
			m.queue = append(m.queue[:i], m.queue[i+1:]...)
			return
		}
	}
}

// removeByPlayerLocked remove um ticket existente do jogador (assume lock).
func (m *Matchmaker) removeByPlayerLocked(playerID string) {
	for i, q := range m.queue {
		if q.playerID == playerID {
			m.queue = append(m.queue[:i], m.queue[i+1:]...)
			return
		}
	}
}

func playerIDs(ts []*ticket) []string {
	ids := make([]string, len(ts))
	for i, t := range ts {
		ids[i] = t.playerID
	}
	return ids
}
