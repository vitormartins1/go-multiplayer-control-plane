// Package service contém o matchmaker: a fila concorrente que forma partidas.
// Só depende das interfaces de domain (ADR-0002). É o "core gordo de
// concorrência" — ver GO-LEARNINGS #01.
package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/c130n/game-backend/services/matchmaking-service/internal/domain"
)

var _ domain.Matchmaker = (*Matchmaker)(nil)

// provisionTimeout limita a chamada de provisionamento ao session por match.
const provisionTimeout = 5 * time.Second

type ticket struct {
	gameID   string // o jogo do ticket — só casa com tickets do mesmo jogo
	playerID string
	ch       chan domain.MatchResult // buffer 1: a entrega nunca bloqueia
}

// Matchmaker mantém a fila de tickets em memória e forma matches FIFO.
type Matchmaker struct {
	matchSize   int
	interval    time.Duration
	provisioner domain.SessionProvisioner

	baseCtx context.Context // ctx do Run, herdado pelas goroutines de provisionamento

	mu            sync.Mutex
	queue         []*ticket
	matchesFormed int64
	matchesFailed int64
}

// New cria o matchmaker (tamanho do match, intervalo do loop e o provisioner
// de sessão usado ao formar cada match).
func New(matchSize int, interval time.Duration, provisioner domain.SessionProvisioner) *Matchmaker {
	return &Matchmaker{
		matchSize:   matchSize,
		interval:    interval,
		provisioner: provisioner,
		baseCtx:     context.Background(),
	}
}

// Join enfileira o jogador (um ticket por jogador) e devolve o canal do Match.
// Quando o ctx é cancelado (stream fechado), o ticket sai da fila.
func (m *Matchmaker) Join(ctx context.Context, gameID, playerID string) (<-chan domain.MatchResult, error) {
	t := &ticket{gameID: gameID, playerID: playerID, ch: make(chan domain.MatchResult, 1)}

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
	m.baseCtx = ctx // goroutines de provisionamento herdam o lifecycle do Run
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

// tryForm agrupa matchSize tickets do MESMO jogo (FIFO por jogo) e, fora da
// seção crítica, dispara uma goroutine por match para provisionar a sessão.
// Jogadores de jogos diferentes nunca entram no mesmo match (ADR-0017).
func (m *Matchmaker) tryForm() {
	var formed [][]*ticket

	m.mu.Lock()
	// agrupa por jogo preservando a ordem de chegada.
	byGame := map[string][]*ticket{}
	for _, t := range m.queue {
		byGame[t.gameID] = append(byGame[t.gameID], t)
	}
	taken := map[*ticket]bool{}
	for _, ts := range byGame {
		for len(ts) >= m.matchSize {
			group := ts[:m.matchSize:m.matchSize]
			ts = ts[m.matchSize:]
			for _, t := range group {
				taken[t] = true
			}
			formed = append(formed, group)
		}
	}
	if len(taken) > 0 {
		kept := m.queue[:0:0]
		for _, t := range m.queue {
			if !taken[t] {
				kept = append(kept, t)
			}
		}
		m.queue = kept
	}
	m.mu.Unlock()

	for _, group := range formed {
		go m.provision(uuid.NewString(), group)
	}
}

// provision chama o session-service e entrega o resultado a cada ticket do match:
// no sucesso, o connect token do PRÓPRIO jogador; na falha, MatchResult{Failed}.
func (m *Matchmaker) provision(matchID string, group []*ticket) {
	players := playerIDs(group)
	gameID := group[0].gameID // o grupo é de um único jogo

	ctx, cancel := context.WithTimeout(m.baseCtx, provisionTimeout)
	defer cancel()
	prov, err := m.provisioner.Provision(ctx, matchID, players, gameID)
	if err != nil {
		m.deliverFailure(group, matchID, players, reasonFor(err))
		return
	}

	for _, t := range group {
		t.ch <- domain.MatchResult{
			MatchID:       matchID,
			Players:       players,
			ServerAddress: prov.ServerAddress,
			ConnectToken:  prov.Tokens[t.playerID], // token do próprio jogador
		}
		close(t.ch)
	}
	m.mu.Lock()
	m.matchesFormed++
	m.mu.Unlock()
}

func (m *Matchmaker) deliverFailure(group []*ticket, matchID string, players []string, reason string) {
	for _, t := range group {
		t.ch <- domain.MatchResult{MatchID: matchID, Players: players, Failed: true, Reason: reason}
		close(t.ch)
	}
	m.mu.Lock()
	m.matchesFailed++
	m.mu.Unlock()
}

// reasonFor traduz o erro de provisionamento numa mensagem curta para o cliente.
func reasonFor(err error) string {
	if errors.Is(err, domain.ErrNoServer) {
		return "no game server available"
	}
	return "session provisioning failed"
}

// QueueSize devolve o tamanho atual da fila (métrica).
func (m *Matchmaker) QueueSize() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.queue)
}

// MatchesTotal devolve o total de matches formados com sucesso (métrica).
func (m *Matchmaker) MatchesTotal() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return float64(m.matchesFormed)
}

// MatchesFailed devolve o total de matches que falharam no provisionamento (métrica).
func (m *Matchmaker) MatchesFailed() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return float64(m.matchesFailed)
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
