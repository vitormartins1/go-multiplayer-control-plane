package domain

import "errors"

// Erros de domínio do game-registry. O transport mapeia cada um para um HTTP
// status code (ADR-0002 — domínio não conhece HTTP).
var (
	ErrInvalidSlug  = errors.New("game-registry: slug inválido")
	ErrInvalidName  = errors.New("game-registry: nome inválido")
	ErrSlugTaken    = errors.New("game-registry: slug já em uso")
	ErrGameNotFound = errors.New("game-registry: jogo não encontrado")

	// Fleet Agones (ADR-0026, spec 012).
	ErrInvalidFleet = errors.New("game-registry: fleet inválida")

	// Descrição do jogo (spec 013).
	ErrInvalidDescription = errors.New("game-registry: descrição inválida (máximo 500 caracteres)")

	// Perfil de multiplayer (ADR-0023, spec 006).
	ErrUnsupportedArchetype = errors.New("game-registry: arquétipo de multiplayer não suportado")
	ErrUnsupportedTopology  = errors.New("game-registry: topologia não suportada")
	ErrInvalidProfileParams = errors.New("game-registry: parâmetros do perfil inválidos")
	ErrProfileNotFound      = errors.New("game-registry: perfil de multiplayer não configurado")

	// Modos nomeados (ADR-0024, spec 009).
	ErrInvalidModeID        = errors.New("game-registry: modeId inválido")
	ErrModeNotFound         = errors.New("game-registry: modo não encontrado")
	ErrDefaultModeProtected = errors.New("game-registry: o modo default não pode ser removido")
	ErrTooManyModes         = errors.New("game-registry: limite de modos do jogo atingido")
	// ErrModeConflict sinaliza controle de concorrência otimista em SetModes
	// (achado no /code-review, revisão do trade-off "last-write-wins aceito"
	// da spec 009/plan.md): outra escrita mudou os modos do jogo entre o
	// FindByID e este SetModes — a mudança local não foi salva, refaça a
	// leitura e tente de novo.
	ErrModeConflict = errors.New("game-registry: modos do jogo mudaram por outra escrita concorrente — recarregue e tente de novo")
)
