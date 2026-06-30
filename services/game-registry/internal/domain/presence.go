package domain

import (
	"context"
	"time"
)

// Presence é a "presença" de um jogador num jogo: o player_id (sub do JWT) e o
// handle (desnormalizado do login, para o painel listar sem lookup no identity).
// Identidade continua global (ADR-0017 #2); aqui só registramos quem "está" no
// jogo (ADR-0019). first_seen é o primeiro login-into-game.
type Presence struct {
	PlayerID  string
	Handle    string
	FirstSeen time.Time
}

// PresenceService é a Port de entrada da presença por jogo.
type PresenceService interface {
	// Mark registra (upsert idempotente) a presença de um jogador num jogo.
	// ErrGameNotFound se o jogo não existir.
	Mark(ctx context.Context, gameID, playerID, handle string) error
	// PlayersOf devolve a base do jogo por first_seen (asc).
	PlayersOf(ctx context.Context, gameID string) ([]Presence, error)
}

// PresenceRepository é a Port de saída de persistência da presença.
type PresenceRepository interface {
	// Upsert insere a presença; re-marcar mantém first_seen e atualiza handle.
	// ErrGameNotFound em violação de FK (jogo inexistente).
	Upsert(ctx context.Context, gameID, playerID, handle string) error
	// ListByGame devolve a base do jogo por first_seen (asc).
	ListByGame(ctx context.Context, gameID string) ([]Presence, error)
}
