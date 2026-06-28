package domain

import "errors"

// Erros de domínio do achievements-service. O transport mapeia cada um para um
// HTTP status code (ver ADR-0002 — domínio não conhece HTTP).
var (
	ErrUnauthorized = errors.New("achievements: não autorizado")
	ErrInvalidEvent = errors.New("achievements: evento inválido")
)
