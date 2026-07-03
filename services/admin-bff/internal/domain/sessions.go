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

// GameSession é uma partida do jogo (aberta ou fechada) para a aba Sessions.
type GameSession struct {
	SessionID     string          `json:"sessionId"`
	MatchID       string          `json:"matchId"`
	State         string          `json:"state"`
	Players       []SessionPlayer `json:"players"`
	ServerAddress string          `json:"serverAddress"`
	OpenedAt      time.Time       `json:"openedAt"`
	ClosedAt      *time.Time      `json:"closedAt,omitempty"`
	CloseReason   string          `json:"closeReason,omitempty"`
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
}
