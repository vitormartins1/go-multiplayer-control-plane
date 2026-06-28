package domain

import "errors"

// Erros de domínio do achievements-service. O transport mapeia cada um para um
// HTTP status code (ver ADR-0002 — domínio não conhece HTTP).
// ErrInvalidEvent é o único erro de domínio; a autorização é tratada pelo
// shared/auth no middleware de transport (mapeada a 401 lá).
var ErrInvalidEvent = errors.New("achievements: evento inválido")
