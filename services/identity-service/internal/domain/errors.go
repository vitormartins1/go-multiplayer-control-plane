package domain

import "errors"

// Erros de domínio do identity-service. O transport mapeia cada um para um
// HTTP status code (ver ADR-0002 — domínio não conhece HTTP).
var (
	ErrInvalidHandle      = errors.New("identity: handle inválido")
	ErrHandleTaken        = errors.New("identity: handle já em uso")
	ErrWeakPassword       = errors.New("identity: senha fraca")
	ErrInvalidCredentials = errors.New("identity: credenciais inválidas")

	// ErrAccountNotFound é um sentinel interno (repository -> service).
	// Nunca é exposto à API: o service o traduz para ErrInvalidCredentials,
	// para não revelar se o handle existe.
	ErrAccountNotFound = errors.New("identity: conta não encontrada")
)
