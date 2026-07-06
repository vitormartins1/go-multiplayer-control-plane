// Package domain contém os tipos de negócio puros e as interfaces (Ports)
// do leaderboard-service. Zero dependências externas. Ver ADR-0002.
package domain

import (
	"context"
	"regexp"
)

// Board é a métrica de um ranking (ADR-0023). Cada jogo tem N rankings, um por
// board: "score" (default) + um por stat do resultado (ex.: "kills").
const BoardScore = "score"

// boardRE valida o nome de um board — mesmo identificador estável do scoringStats
// no game-registry (`^[a-z][a-z0-9_]*$`).
var boardRE = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// ValidBoard diz se o nome de board é aceitável.
func ValidBoard(board string) bool { return boardRE.MatchString(board) }

// Entry é uma posição no ranking. Rank é 1-based (para exibição).
type Entry struct {
	Rank     int64
	PlayerID string
	Score    int64
}

// LeaderboardService é a Port de entrada do leaderboard-service. Tudo é escopado
// por (gameID, board) — ranking por jogo (ADR-0017) e por métrica (ADR-0023).
type LeaderboardService interface {
	// Submit aplica highscore e retorna a entry do jogador após a submissão.
	Submit(ctx context.Context, gameID, board, playerID string, score int64) (Entry, error)
	// ForceSubmit substitui o score independente do valor atual (admin override).
	ForceSubmit(ctx context.Context, gameID, board, playerID string, score int64) (Entry, error)
	Top(ctx context.Context, gameID, board string, limit int) ([]Entry, error)
	Around(ctx context.Context, gameID, board, playerID string, k int) ([]Entry, error)
}

// LeaderboardStore é a Port de saída (implementada por um adapter Redis). Cada
// (jogo, board) é um sorted set próprio (key por gameID+board).
type LeaderboardStore interface {
	Submit(ctx context.Context, gameID, board, playerID string, score int64) error // highscore (ZADD GT)
	// ForceScore substitui o score independente do valor atual (ZADD sem GT).
	ForceScore(ctx context.Context, gameID, board, playerID string, score int64) error
	// Rank devolve a posição (0-based, ordem do Redis) e o score do jogador;
	// ErrNotRanked se ele ainda não tem score.
	Rank(ctx context.Context, gameID, board, playerID string) (rank int64, score int64, err error)
	Top(ctx context.Context, gameID, board string, limit int) ([]Entry, error)
	// Range devolve as entradas das posições [start, stop] (0-based, inclusivo).
	Range(ctx context.Context, gameID, board string, start, stop int64) ([]Entry, error)
}

// TokenVerifier valida o JWT do identity e devolve o playerID (sub).
// Implementada por shared/auth (JWKS).
type TokenVerifier interface {
	Verify(ctx context.Context, token string) (playerID string, err error)
}
