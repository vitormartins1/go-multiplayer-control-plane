package domain

import (
	"context"
	"errors"
	"time"
)

// ErrSessionsUnavailable indica que o session-service está inacessível. O
// transport mapeia -> 502.
var ErrSessionsUnavailable = errors.New("admin: sessions source unavailable")

// SessionPlayer é um jogador da sessão com o handle resolvido no identity
// (best-effort: identity fora -> handle vazio, o id sempre presente).
type SessionPlayer struct {
	ID     string `json:"id"`
	Handle string `json:"handle"`
}

// GameSession é uma partida (aberta ou fechada) para a aba Sessions e o board
// global Rooms. GameID/GameName ficam vazios na aba por-jogo (já se sabe o jogo)
// e preenchidos no board global (spec 008).
type GameSession struct {
	SessionID     string          `json:"sessionId"`
	MatchID       string          `json:"matchId"`
	GameID        string          `json:"gameId,omitempty"`
	GameName      string          `json:"gameName,omitempty"`
	State         string          `json:"state"`
	Players       []SessionPlayer `json:"players"`
	ServerAddress string          `json:"serverAddress"`
	OpenedAt      time.Time       `json:"openedAt"`
	ClosedAt      *time.Time      `json:"closedAt,omitempty"`
	CloseReason   string          `json:"closeReason,omitempty"`

	// host_world_dropin (spec 014): origem, host e slots por papel (co-op X/2,
	// invasor Y/1). origin sempre presente; os demais só em mundos host_world.
	Origin       string     `json:"origin,omitempty"`
	HostPlayerID string     `json:"hostPlayerId,omitempty"`
	RoleSlots    []RoleSlot `json:"roleSlots,omitempty"`
}

// RoleSlot é a ocupação de um papel num mundo host_world (spec 014).
type RoleSlot struct {
	Role     string `json:"role"`
	Capacity int    `json:"capacity"`
	Occupied int    `json:"occupied"`
}

// GameSessions agrupa as sessões do jogo como o painel consome.
type GameSessions struct {
	Active []GameSession `json:"active"`
	Closed []GameSession `json:"closed"`
}

// SessionsReader é a Port de saída da API admin do session-service.
type SessionsReader interface {
	// SessionsByGame lê as sessões do jogo; state ("open"|"closed"|"") é
	// repassado como filtro. Indisponibilidade -> ErrSessionsUnavailable.
	SessionsByGame(ctx context.Context, gameID, state string) (GameSessions, error)
	// AllSessions lê as sessões de todos os jogos (board global do console, 008).
	AllSessions(ctx context.Context, state string) (GameSessions, error)
	// CloseSession encerra a sessão ativa (force-close do operador). 404 do
	// session -> UpstreamError{Status:404}.
	CloseSession(ctx context.Context, matchID string) (GameSession, error)
}

// FanoutHealth são os sinais do fan-out durável (ADR-0020) via PromQL. Ponteiros:
// nil = métrica ausente/Prometheus fora (não confundir com zero), como LiveMetrics.
type FanoutHealth struct {
	PublishOK    *float64 `json:"publishOk"`
	PublishError *float64 `json:"publishError"`
	ConsumeOK    *float64 `json:"consumeOk"`
	ConsumeDup   *float64 `json:"consumeDup"`
	ConsumeError *float64 `json:"consumeError"`
}
