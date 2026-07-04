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
//
// Capa (feature 003-generated-cover): sem upload no MVP, a imagem é gerada no
// cliente a partir de GeneratedImageSeed (default = slug) no estilo
// GeneratedImageStyle. Image (URL/data-uri) e ImageSource ("generated" | futuro
// "upload") deixam o espaço para upload futuro.
type Game struct {
	ID                  string
	Slug                string
	Name                string
	CreatedAt           time.Time
	Image               string // vazio = sem upload (reservado)
	ImageSource         string // "generated" no MVP
	GeneratedImageSeed  string // seed determinística (default = slug)
	GeneratedImageStyle string // estilo do gerador (default = "shapes")
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
	// Rename altera só o nome do jogo (slug/id são estáveis). ErrInvalidName em
	// entrada inválida; ErrGameNotFound se não existir.
	Rename(ctx context.Context, id, name string) (*Game, error)
}

// GameRepository é a Port de saída de persistência (implementada em repository).
type GameRepository interface {
	// Create persiste o jogo; ErrSlugTaken em violação de unicidade do slug.
	Create(ctx context.Context, g *Game) error
	// List devolve os jogos por nome (asc).
	List(ctx context.Context) ([]Game, error)
	// FindByID busca por id; ErrGameNotFound se ausente.
	FindByID(ctx context.Context, id string) (*Game, error)
	// UpdateName atualiza só o nome do jogo; ErrGameNotFound se não existir.
	UpdateName(ctx context.Context, id, name string) error
}
