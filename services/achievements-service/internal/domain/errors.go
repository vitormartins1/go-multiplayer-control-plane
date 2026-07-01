package domain

import "errors"

// Erros de domínio do achievements-service. O transport mapeia cada um para um
// HTTP status code (ver ADR-0002 — domínio não conhece HTTP).
// A autorização é tratada pelo shared/auth no middleware de transport (401 lá).
var (
	ErrInvalidEvent = errors.New("achievements: evento inválido")
	// ErrInvalidAchievement: slug/name/target inválidos na criação (admin).
	ErrInvalidAchievement = errors.New("achievements: conquista inválida")
	// ErrAchievementExists: id (slug) já existe no jogo.
	ErrAchievementExists = errors.New("achievements: conquista já existe no jogo")
	// ErrAchievementNotFound: conquista (game_id, id) não existe (update/delete).
	ErrAchievementNotFound = errors.New("achievements: conquista não encontrada")
)
