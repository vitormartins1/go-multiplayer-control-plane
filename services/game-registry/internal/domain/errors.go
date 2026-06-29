package domain

import "errors"

// Erros de domínio do game-registry. O transport mapeia cada um para um HTTP
// status code (ADR-0002 — domínio não conhece HTTP).
var (
	ErrInvalidSlug  = errors.New("game-registry: slug inválido")
	ErrInvalidName  = errors.New("game-registry: nome inválido")
	ErrSlugTaken    = errors.New("game-registry: slug já em uso")
	ErrGameNotFound = errors.New("game-registry: jogo não encontrado")
)
