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

	// Erros do arquétipo host_world_dropin (ADR-0023, spec 011).

	// ErrUnsupportedArchetype indica que o jogo não é host_world_dropin — só esse
	// arquétipo abre mundo (OpenWorld) sem match prévio.
	ErrUnsupportedArchetype = errors.New("session: unsupported archetype for host world")
	// ErrRoleNotEnabled indica que o papel pedido não está habilitado no perfil do
	// jogo (fora de EnabledRoles).
	ErrRoleNotEnabled = errors.New("session: role not enabled")
	// ErrRoleFull indica que os slots do papel no mundo estão esgotados.
	ErrRoleFull = errors.New("session: role slots full")
	// ErrWorldNotFound indica que não há mundo (sessão host_world) aberto com o id.
	ErrWorldNotFound = errors.New("session: world not found")
	// ErrAlreadyJoined indica que o jogador já ocupa um slot no mundo.
	ErrAlreadyJoined = errors.New("session: player already joined world")
	// ErrProfileUnavailable indica que o perfil de multiplayer do jogo não pôde ser
	// lido/usado (ausente, arquétipo incompatível, rede) — origem no ProfileProvider.
	ErrProfileUnavailable = errors.New("session: multiplayer profile unavailable")
	// ErrInvalidParticipation indica um evento de participação fora do vocabulário
	// (Event/Outcome inválido) — spec 013.
	ErrInvalidParticipation = errors.New("session: invalid participation event")

	// ErrOutcomeMismatch indica que o kind do outcome não corresponde ao arquétipo
	// do snapshot da sessão (ADR-0024, spec 017) — ex.: um server de queue_arena
	// reportando collective. O resultado NÃO é publicado.
	ErrOutcomeMismatch = errors.New("session: outcome kind does not match session archetype")
)
