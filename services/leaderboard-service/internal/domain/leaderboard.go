// Package domain contém os tipos de negócio puros e as interfaces (Ports)
// do leaderboard-service. Zero dependências externas. Ver ADR-0002.
package domain

import "context"

// Entry é uma posição no ranking. Rank é 1-based (para exibição).
type Entry struct {
	Rank     int64
	PlayerID string
	Score    int64
}

// LeaderboardService é a Port de entrada do leaderboard-service. Tudo é
// escopado por gameID (ranking por jogo, ADR-0017).
type LeaderboardService interface {
	// Submit aplica highscore e retorna a entry do jogador após a submissão.
	Submit(ctx context.Context, gameID, playerID string, score int64) (Entry, error)
	// ForceSubmit substitui o score independente do valor atual (admin override).
	ForceSubmit(ctx context.Context, gameID, playerID string, score int64) (Entry, error)
	Top(ctx context.Context, gameID string, limit int) ([]Entry, error)
	Around(ctx context.Context, gameID, playerID string, k int) ([]Entry, error)
}

// LeaderboardStore é a Port de saída (implementada por um adapter Redis). Cada
// jogo é um sorted set próprio (key por gameID).
type LeaderboardStore interface {
	Submit(ctx context.Context, gameID, playerID string, score int64) error // highscore (ZADD GT)
	// ForceScore substitui o score independente do valor atual (ZADD sem GT).
	ForceScore(ctx context.Context, gameID, playerID string, score int64) error
	// Rank devolve a posição (0-based, ordem do Redis) e o score do jogador;
	// ErrNotRanked se ele ainda não tem score.
	Rank(ctx context.Context, gameID, playerID string) (rank int64, score int64, err error)
	Top(ctx context.Context, gameID string, limit int) ([]Entry, error)
	// Range devolve as entradas das posições [start, stop] (0-based, inclusivo).
	Range(ctx context.Context, gameID string, start, stop int64) ([]Entry, error)
}

// TokenVerifier valida o JWT do identity e devolve o playerID (sub).
// Implementada por shared/auth (JWKS).
type TokenVerifier interface {
	Verify(ctx context.Context, token string) (playerID string, err error)
}
