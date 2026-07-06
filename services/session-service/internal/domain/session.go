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
	// CloseAdmin: encerrada por um operador no console (spec 007).
	CloseAdmin CloseReason = "admin"
	// CloseAbandoned: fechada pela reconciliação — o game server sumiu da fleet
	// sem ReportResult (spec 008, ADR-0022).
	CloseAbandoned CloseReason = "abandoned"
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

// Vocabulário do outcome (ADR-0023). No MVP só o kind win_loss_draw existe.
const (
	OutcomeWinLossDraw = "win_loss_draw"

	ResultWin  = "win"
	ResultLoss = "loss"
	ResultDraw = "draw"
)

// MatchOutcome é o resultado da partida, polimórfico por arquétipo (ADR-0023),
// modelado como tagged union por ponteiro: exatamente o ponteiro do Kind
// declarado é não-nil. No MVP só WinLossDraw (arquétipo queue_arena); novos
// arquétipos adicionam um campo-ponteiro próprio (placement, collective, ...).
type MatchOutcome struct {
	Kind        string // "win_loss_draw" (único no MVP)
	WinLossDraw *WinLossDrawResult
}

// WinLossDrawResult é o outcome do queue_arena: vitória/derrota/empate por
// jogador (e por time, via TeamID), com stats.
type WinLossDrawResult struct {
	Players []PlayerOutcome
}

// PlayerOutcome é o resultado de um jogador na partida (reportado pelo server).
type PlayerOutcome struct {
	PlayerID string
	TeamID   string // vazio em FFA
	Outcome  string // "win" | "loss" | "draw"
	Score    int64
	Stats    map[string]int64 // métricas declaradas (kills, assists, ...); pode ser nil
}

// SessionService é a Port de entrada: cria sessões a partir de matches e
// recebe o resultado da partida.
type SessionService interface {
	// Create aloca um game server, emite um connect token por jogador e registra
	// a sessão. Devolve ErrNoServer se o pool esgotar.
	Create(ctx context.Context, matchID, gameID string, players []string) (*Session, []ConnectGrant, error)
	// ReportResult dispara o fan-out do resultado, libera o server e encerra a
	// sessão. ErrUnsupportedOutcome/ErrInvalidOutcome em outcome inválido;
	// ErrSessionNotFound se não houver sessão para o match.
	ReportResult(ctx context.Context, matchID string, outcome MatchOutcome) error
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

// ServerRegistry é a Port de saída OPCIONAL que consulta a liveness do pool:
// quais game servers ainda existem na fonte de verdade. Só o adapter Agones a
// implementa (LIST de GameServers da fleet); o static não (seus servers nunca
// somem). Quando o Service recebe registry nil, não reconcilia — cai só no TTL
// sweeper (spec 008, ADR-0022).
type ServerRegistry interface {
	// LiveServerIDs devolve o conjunto de IDs de game servers existentes na
	// fleet. O ID é o mesmo que Allocate grava em Server.ID.
	LiveServerIDs(ctx context.Context) (map[string]struct{}, error)
}

// ConnectTokenIssuer é a Port de saída que cunha o connect token (JWT EdDSA)
// validado offline pelo game server via JWKS (ADR-0010/0011).
type ConnectTokenIssuer interface {
	// Issue emite um JWT EdDSA com sub/match_id/server/exp.
	Issue(playerID, matchID, serverAddr string) (token string, err error)
	// JWKS devolve a chave pública (JSON) para verificação offline.
	JWKS(ctx context.Context) ([]byte, error)
}

// ResultPublisher é a Port de saída do fan-out do resultado. O adapter real
// (SNSPublisher) publica o resultado bruto no tópico match-results; cada sink
// consome sua fila SQS com idempotência por matchID (ADR-0020). LogPublisher é
// o fallback de dev. Recebe gameID (multi-tenant, ADR-0017) e matchID (chave de
// idempotência que viaja no envelope).
type ResultPublisher interface {
	Publish(ctx context.Context, gameID, matchID string, outcome MatchOutcome) error
}
