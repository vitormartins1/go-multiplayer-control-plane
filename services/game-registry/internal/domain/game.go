// Package domain contém os tipos de negócio puros e as interfaces (Ports) do
// game-registry. Zero dependências externas. Ver ADR-0002.
//
// O game-registry é a fonte canônica de jogos (ADR-0017): fornece o game_id
// (UUID) que os demais domínios usam como dimensão de multi-tenant.
package domain

import (
	"context"
	"time"
)

// Game é um jogo registrado no backend. O id (UUID) é a chave canônica usada
// pelos outros serviços; o slug é a chave humana estável.
type Game struct {
	ID        string
	Slug      string
	Name      string
	CreatedAt time.Time
}

// GameService é a Port de entrada do game-registry.
type GameService interface {
	// Register valida e cria um jogo. ErrInvalidSlug/ErrInvalidName em entrada
	// inválida; ErrSlugTaken se o slug já existir.
	Register(ctx context.Context, slug, name string) (*Game, error)
	// List devolve os jogos por nome (asc).
	List(ctx context.Context) ([]Game, error)
	// Get devolve um jogo por id; ErrGameNotFound se ausente.
	Get(ctx context.Context, id string) (*Game, error)
}

// GameRepository é a Port de saída de persistência (implementada em repository).
type GameRepository interface {
	// Create persiste o jogo; ErrSlugTaken em violação de unicidade do slug.
	Create(ctx context.Context, g *Game) error
	// List devolve os jogos por nome (asc).
	List(ctx context.Context) ([]Game, error)
	// FindByID busca por id; ErrGameNotFound se ausente.
	FindByID(ctx context.Context, id string) (*Game, error)
}
