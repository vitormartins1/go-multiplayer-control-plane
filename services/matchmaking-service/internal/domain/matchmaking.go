// Package domain contém os tipos de negócio puros e as interfaces (Ports)
// do matchmaking-service. Zero dependências externas. Ver ADR-0002.
package domain

import "context"

// Match é uma partida formada a partir de tickets da fila.
type Match struct {
	ID      string
	Players []string
}

// MatchResult é o que cada jogador recebe pelo seu canal quando o match resolve:
// ou o sucesso (server + o connect token do PRÓPRIO jogador), ou a falha de
// provisionamento da sessão (com o motivo). Cada jogador recebe um token distinto.
type MatchResult struct {
	MatchID       string
	Players       []string
	ServerAddress string // preenchido no sucesso
	ConnectToken  string // connect token do jogador deste ticket (sucesso)
	Failed        bool   // true se o provisionamento da sessão falhou
	Reason        string // motivo, quando Failed
}

// Provisioned é o resultado de provisionar uma sessão no session-service:
// o endereço do game server e o connect token de cada jogador (por playerID).
type Provisioned struct {
	ServerAddress string
	Tokens        map[string]string // playerID -> connect token
}

// Estruturas de formação do arquétipo queue_arena (ADR-0023).
const (
	StructureFFA   = "ffa"
	StructureTeams = "teams"
)

// MatchFormat é a forma da partida derivada do perfil de multiplayer do jogo
// (ADR-0023, queue_arena): o tamanho (critério de agrupamento) e, em teams, a
// composição de lados. Size = playerCount (ffa) ou teamCount*teamSize (teams).
type MatchFormat struct {
	Size      int
	Structure string // "ffa" | "teams"
	TeamCount int    // >0 só em teams
	TeamSize  int    // >0 só em teams
}

// TeamAssignment é um lado da partida NvN: um rótulo estável no match e seus
// jogadores (ADR-0023). Em FFA a composição é um único assignment com TeamID
// vazio e todos os jogadores.
type TeamAssignment struct {
	TeamID  string
	Players []string
}

// Matchmaker é a Port de entrada: enfileira tickets e forma matches. O
// gameID escopa a fila — só casa jogadores do mesmo jogo (ADR-0017).
type Matchmaker interface {
	// Join enfileira o jogador no jogo e devolve um canal por onde o MatchResult
	// chega quando o match resolve. Cancelar o ctx remove o ticket da fila.
	Join(ctx context.Context, gameID, playerID string) (<-chan MatchResult, error)
	// Run executa o loop de formação (em background) até o ctx ser cancelado.
	Run(ctx context.Context)
}

// SessionProvisioner é a Port de saída para o session-service: ao formar um
// match, provisiona a sessão (aloca server + emite os connect tokens) para o
// jogo, levando a composição de times (ADR-0023). Em FFA, um único
// TeamAssignment com TeamID vazio. Devolve ErrNoServer quando o session
// responde ResourceExhausted.
type SessionProvisioner interface {
	Provision(ctx context.Context, matchID string, teams []TeamAssignment, gameID string) (Provisioned, error)
}

// ProfileProvider é a Port de saída que devolve a formação do match derivada do
// perfil de multiplayer do jogo (ADR-0023, arquétipo queue_arena): o tamanho
// (ffa -> playerCount, teams -> teamCount*teamSize) e a composição de lados. A
// derivação mora no adapter; o domínio recebe o MatchFormat pronto. Devolve
// ErrProfileUnavailable quando o perfil não está disponível/utilizável.
type ProfileProvider interface {
	Format(ctx context.Context, gameID string) (MatchFormat, error)
}

// TokenVerifier valida o JWT do identity e devolve o playerID (sub).
// Implementada por shared/auth (JWKS), usada no interceptor gRPC.
type TokenVerifier interface {
	Verify(ctx context.Context, token string) (playerID string, err error)
}
