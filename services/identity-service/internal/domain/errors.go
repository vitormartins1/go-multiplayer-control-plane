package domain

import "errors"

// Erros de domínio do identity-service. O transport mapeia cada um para um
// HTTP status code (ver ADR-0002 — domínio não conhece HTTP).
var (
	ErrInvalidHandle      = errors.New("identity: handle inválido")
	ErrHandleTaken        = errors.New("identity: handle já em uso")
	ErrWeakPassword       = errors.New("identity: senha fraca")
	ErrInvalidCredentials = errors.New("identity: credenciais inválidas")
)
