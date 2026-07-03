// Package domain contém os tipos de negócio puros e as interfaces (Ports) do
// session-service. Zero dependências externas. Ver ADR-0002.
package domain

import (
	"context"
	"time"
)

// SessionState é o estado do ciclo de vida da sessão (spec 004).
type SessionState string

// CloseReason é o motivo do fechamento da sessão (spec 004).
type CloseReason string

const (
	SessionOpen   SessionState = "open"
	SessionClosed SessionState = "closed"

	// CloseResult: fechada por ReportResult (fim de partida reportado).
	CloseResult CloseReason = "result"
	// CloseTTL: fechada pelo sweeper (aberta há mais que SESSION_TTL).
	CloseTTL CloseReason = "ttl"
)

// Session é uma partida em andamento: o match alocado a um game server.
type Session struct {
	ID            string
	MatchID       string
	GameID        string // jogo da partida (multi-tenant, ADR-0017)
	Players       []string
	ServerID      string
	ServerAddress string

	State       SessionState
	OpenedAt    time.Time
	ClosedAt    time.Time // zero enquanto OPEN
	CloseReason CloseReason
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
	Create(ctx context.Context, matchID, gameID string, players []string) (*Session, []ConnectGrant, error)
	// ReportResult dispara o fan-out do resultado, libera o server e encerra a
	// sessão. Devolve ErrSessionNotFound se não houver sessão para o match.
	ReportResult(ctx context.Context, matchID string, results []PlayerResult) error
}

// ServerAllocator é a Port de saída do pool de game servers. O adapter static
// aloca de um pool fixo em memória; o Agones cria uma GameServerAllocation
// (ADR-0011, spec 004) levando os metadados da partida para o GameServer.
type ServerAllocator interface {
	// Allocate reserva um game server livre para a partida. matchID/gameID viram
	// annotations do GameServer no Agones (o static ignora). ErrNoServer se o
	// pool esgotar.
	Allocate(ctx context.Context, matchID, gameID string) (Server, error)
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

// ResultPublisher é a Port de saída do fan-out do resultado para os serviços de
// dados (achievements/...). Recebe o gameID da sessão (multi-tenant, ADR-0017).
// O adapter real fala HTTP com a internal key (ADR-0018); LogPublisher é fallback.
type ResultPublisher interface {
	Publish(ctx context.Context, gameID string, results []PlayerResult) error
}
