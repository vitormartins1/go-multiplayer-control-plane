// Package service contém o matchmaker: a fila concorrente que forma partidas.
// Só depende das interfaces de domain (ADR-0002). É o "core gordo de
// concorrência" — ver GO-LEARNINGS #01.
package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/vitormartins1/go-game-backend/services/matchmaking-service/internal/domain"
)

var _ domain.Matchmaker = (*Matchmaker)(nil)

// provisionTimeout limita a chamada de provisionamento ao session por match.
const provisionTimeout = 5 * time.Second

type ticket struct {
	gameID   string // o jogo do ticket — só casa com tickets do mesmo jogo
	playerID string
	format   domain.MatchFormat      // formação do match, derivada do perfil no Join (ADR-0023)
	ch       chan domain.MatchResult // buffer 1: a entrega nunca bloqueia
}

// Matchmaker mantém a fila de tickets em memória e forma matches FIFO. O tamanho
// do match não é global: vem do perfil de cada jogo (domain.ProfileProvider),
// resolvido no Join e carimbado no ticket (ADR-0023).
type Matchmaker struct {
	interval    time.Duration
	provider    domain.ProfileProvider
	provisioner domain.SessionProvisioner

	baseCtx context.Context // ctx do Run, herdado pelas goroutines de provisionamento

	mu            sync.Mutex
	queue         []*ticket
	matchesFormed int64
	matchesFailed int64
}

// New cria o matchmaker (intervalo do loop, o provider de perfil que dita o
// tamanho do match por jogo, e o provisioner de sessão usado ao formar cada match).
func New(interval time.Duration, provider domain.ProfileProvider, provisioner domain.SessionProvisioner) *Matchmaker {
	return &Matchmaker{
		interval:    interval,
		provider:    provider,
		provisioner: provisioner,
		baseCtx:     context.Background(),
	}
}

// Join enfileira o jogador (um ticket por jogador) e devolve o canal do Match.
// Quando o ctx é cancelado (stream fechado), o ticket sai da fila.
func (m *Matchmaker) Join(ctx context.Context, gameID, playerID string) (<-chan domain.MatchResult, error) {
	// A formação do match vem do perfil do jogo (ADR-0023). Resolver antes de
	// enfileirar: se o jogo não tem perfil utilizável, o ticket não entra na fila
	// (ErrProfileUnavailable). Fora da seção crítica — nada de I/O sob o lock.
	format, err := m.provider.Format(ctx, gameID)
	if err != nil {
		return nil, err
	}

	t := &ticket{gameID: gameID, playerID: playerID, format: format, ch: make(chan domain.MatchResult, 1)}

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
		// tickets do mesmo jogo compartilham a formação (derivada do perfil no Join).
		size := ts[0].format.Size
		for len(ts) >= size {
			group := ts[:size:size]
			ts = ts[size:]
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
	teams := composeTeams(group)

	ctx, cancel := context.WithTimeout(m.baseCtx, provisionTimeout)
	defer cancel()
	prov, err := m.provisioner.Provision(ctx, matchID, teams, gameID)
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

// composeTeams particiona o grupo em lados conforme a formação do jogo
// (ADR-0023). Atribuição posicional (ordem FIFO), sem skill/MMR (backlog).
//   - teams (queue_arena): blocos contíguos de teamSize, rótulos "team-1".."team-N".
//     O grupo tem exatamente Size = TeamCount*TeamSize tickets (garantido por tryForm).
//   - squads (battle_royale): blocos de teamSize, rótulos "squad-1".."squad-K".
//     lobbySize pode não ser múltiplo de teamSize → o último squad é "ragged"
//     (menor). K = ⌈len/teamSize⌉.
//   - ffa (queue_arena FFA e battle_royale solo): um único assignment sem TeamID
//     com todos os jogadores.
func composeTeams(group []*ticket) []domain.TeamAssignment {
	format := group[0].format
	switch format.Structure {
	case domain.StructureTeams:
		teams := make([]domain.TeamAssignment, 0, format.TeamCount)
		for k := 0; k < format.TeamCount; k++ {
			members := playerIDs(group[k*format.TeamSize : (k+1)*format.TeamSize])
			teams = append(teams, domain.TeamAssignment{TeamID: fmt.Sprintf("team-%d", k+1), Players: members})
		}
		return teams
	case domain.StructureSquads:
		var squads []domain.TeamAssignment
		for i, k := 0, 1; i < len(group); i, k = i+format.TeamSize, k+1 {
			end := i + format.TeamSize
			if end > len(group) {
				end = len(group) // último squad ragged
			}
			squads = append(squads, domain.TeamAssignment{TeamID: fmt.Sprintf("squad-%d", k), Players: playerIDs(group[i:end])})
		}
		return squads
	default: // ffa
		return []domain.TeamAssignment{{Players: playerIDs(group)}}
	}
}
