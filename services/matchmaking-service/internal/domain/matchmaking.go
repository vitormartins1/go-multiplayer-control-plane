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
// jogo. Devolve ErrNoServer quando o session responde ResourceExhausted.
type SessionProvisioner interface {
	Provision(ctx context.Context, matchID string, players []string, gameID string) (Provisioned, error)
}

// ProfileProvider é a Port de saída que devolve o tamanho do match derivado do
// perfil de multiplayer do jogo (ADR-0023, arquétipo queue_arena): ffa ->
// playerCount, teams -> teamCount*teamSize. A derivação mora no adapter; o
// domínio só recebe o tamanho. Devolve ErrProfileUnavailable quando o perfil
// não está disponível/utilizável.
type ProfileProvider interface {
	MatchSize(ctx context.Context, gameID string) (int, error)
}

// TokenVerifier valida o JWT do identity e devolve o playerID (sub).
// Implementada por shared/auth (JWKS), usada no interceptor gRPC.
type TokenVerifier interface {
	Verify(ctx context.Context, token string) (playerID string, err error)
}
