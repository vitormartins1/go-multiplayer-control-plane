// Package domain contém os tipos de negócio puros e as interfaces (Ports) do
// session-service. Zero dependências externas. Ver ADR-0002.
package domain

import "context"

// Session é uma partida em andamento: o match alocado a um game server.
type Session struct {
	ID            string
	MatchID       string
	Players       []string
	ServerID      string
	ServerAddress string
}

// Server é um game server do pool (mock no MVP; Agones no deploy — ADR-0011).
type Server struct {
	ID      string
	Address string
}

// ConnectGrant é o connect token (JWT EdDSA) de um jogador para o game server.
type ConnectGrant struct {
	PlayerID string
	Token    string
}

// PlayerResult é o resultado de um jogador na partida (reportado pelo server).
type PlayerResult struct {
	PlayerID string
	Score    int64
	Won      bool
}

// SessionService é a Port de entrada: cria sessões a partir de matches e
// recebe o resultado da partida.
type SessionService interface {
	// Create aloca um game server, emite um connect token por jogador e registra
	// a sessão. Devolve ErrNoServer se o pool esgotar.
	Create(ctx context.Context, matchID string, players []string) (*Session, []ConnectGrant, error)
	// ReportResult dispara o fan-out do resultado, libera o server e encerra a
	// sessão. Devolve ErrSessionNotFound se não houver sessão para o match.
	ReportResult(ctx context.Context, matchID string, results []PlayerResult) error
}

// ServerAllocator é a Port de saída do pool de game servers. O adapter mock
// aloca de um pool fixo em memória; o real (Agones) entra no deploy (ADR-0011).
type ServerAllocator interface {
	// Allocate reserva um game server livre. ErrNoServer se o pool esgotar.
	Allocate(ctx context.Context) (Server, error)
	// Release devolve o server ao pool, liberando-o para nova alocação.
	Release(ctx context.Context, serverID string) error
}

// ConnectTokenIssuer é a Port de saída que cunha o connect token (JWT EdDSA)
// validado offline pelo game server via JWKS (ADR-0010/0011).
type ConnectTokenIssuer interface {
	// Issue emite um JWT EdDSA com sub/match_id/server/exp.
	Issue(playerID, matchID, serverAddr string) (token string, err error)
	// JWKS devolve a chave pública (JSON) para verificação offline.
	JWKS(ctx context.Context) ([]byte, error)
}

// ResultPublisher é a Port de saída do fan-out do resultado para os serviços
// de dados. Adapter stub que loga no MVP; o real (HTTP + service-auth) é a
// próxima feature.
type ResultPublisher interface {
	Publish(ctx context.Context, matchID string, results []PlayerResult) error
}
