package domain

import "errors"

var (
	// ErrNoServer indica que o pool de game servers está esgotado.
	ErrNoServer = errors.New("session: no server available")
	// ErrSessionNotFound indica que não existe sessão ativa para o match.
	ErrSessionNotFound = errors.New("session: session not found")
)
