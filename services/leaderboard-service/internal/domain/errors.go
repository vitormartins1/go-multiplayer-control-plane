package domain

import "errors"

// Erros de domínio do leaderboard-service. O transport mapeia cada um para um
// HTTP status code (ver ADR-0002). A autorização é tratada pelo shared/auth no
// middleware (401 lá).
var (
	ErrNotRanked    = errors.New("leaderboard: jogador sem score")
	ErrInvalidScore = errors.New("leaderboard: score inválido")
	ErrInvalidBoard = errors.New("leaderboard: board inválido")
)
