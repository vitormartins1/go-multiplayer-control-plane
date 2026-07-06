package domain

import "errors"

var (
	// ErrNoServer indica que o pool de game servers está esgotado.
	ErrNoServer = errors.New("session: no server available")
	// ErrSessionNotFound indica que não existe sessão ativa para o match.
	ErrSessionNotFound = errors.New("session: session not found")
	// ErrUnsupportedOutcome indica um kind de outcome não suportado (ADR-0023).
	ErrUnsupportedOutcome = errors.New("session: unsupported outcome kind")
	// ErrInvalidOutcome indica um outcome estruturalmente inválido (sem jogadores
	// ou com valor fora de win/loss/draw).
	ErrInvalidOutcome = errors.New("session: invalid outcome")
	// ErrInvalidTeams indica uma composição de times inconsistente na criação da
	// sessão (ADR-0023): jogador em dois times, ou divergência entre a lista de
	// players e os jogadores dos times.
	ErrInvalidTeams = errors.New("session: invalid team composition")
)
