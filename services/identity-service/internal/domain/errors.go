package domain

import "errors"

// Erros de domínio do identity-service. O transport mapeia cada um para um
// HTTP status code (ver ADR-0002 — domínio não conhece HTTP).
var (
	ErrInvalidHandle      = errors.New("identity: handle inválido")
	ErrHandleTaken        = errors.New("identity: handle já em uso")
	ErrWeakPassword       = errors.New("identity: senha fraca")
	ErrInvalidCredentials = errors.New("identity: credenciais inválidas")

	// ErrAccountNotFound nasce no repository. No fluxo de login o service o
	// traduz para ErrInvalidCredentials (não revelar se o handle existe); na
	// superfície admin (feature 004) ele chega ao transport e vira 404.
	ErrAccountNotFound = errors.New("identity: conta não encontrada")

	// Identidade federada (spec 005-steam-login).
	ErrProviderNotSupported = errors.New("identity: provedor de login não suportado ou não configurado")
	ErrInvalidTicket        = errors.New("identity: ticket do provedor inválido")
	ErrExternalBanned       = errors.New("identity: jogador banido pelo publisher")
	ErrProviderUnavailable  = errors.New("identity: provedor de login indisponível")
	ErrExternalLinked       = errors.New("identity: identidade externa já vinculada")
)
