// Package domain contém os tipos de negócio puros e as interfaces (Ports)
// do matchmaking-service. Zero dependências externas. Ver ADR-0002.
package domain

import "context"

// Match é uma partida formada a partir de tickets da fila.
type Match struct {
	ID          string
	Players     []string
	ConnectInfo string // mock até o session-service existir
}

// Matchmaker é a Port de entrada: enfileira tickets e forma matches.
type Matchmaker interface {
	// Join enfileira o jogador e devolve um canal por onde o Match chega quando
	// formado. Cancelar o ctx remove o ticket da fila.
	Join(ctx context.Context, playerID string) (<-chan Match, error)
	// Run executa o loop de formação (em background) até o ctx ser cancelado.
	Run(ctx context.Context)
}

// TokenVerifier valida o JWT do identity e devolve o playerID (sub).
// Implementada por shared/auth (JWKS), usada no interceptor gRPC.
type TokenVerifier interface {
	Verify(ctx context.Context, token string) (playerID string, err error)
}
