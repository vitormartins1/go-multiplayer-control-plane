// Package service contém as regras de negócio do game-registry: valida e
// registra jogos. Só depende das interfaces de domain (ADR-0002).
package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/game-registry/internal/domain"
)

var _ domain.GameService = (*service)(nil)

// slugRE: 2–40 chars, minúsculas/dígitos/hífen, sem hífen nas pontas (as duas
// pontas são alfanuméricas, então o mínimo é 2 caracteres).
var slugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,38}[a-z0-9]$`)

// fleetRE: nome de Fleet Agones (label agones.dev/fleet) — mesmo formato de
// modeIDRE (profile_service.go), compatível com label do Kubernetes.
var fleetRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

const maxNameLen = 100

// defaultImageStyle é o estilo do gerador de capa (cliente). Trocar aqui muda
// o default de novos cadastros; o front mapeia o nome -> coleção (ADR/spec 003).
const defaultImageStyle = "shapes"

type service struct {
	repo domain.GameRepository
}

// New cria o GameService do game-registry.
func New(repo domain.GameRepository) domain.GameService {
	return &service{repo: repo}
}

func (s *service) Register(ctx context.Context, slug, name string) (*domain.Game, error) {
	if !slugRE.MatchString(slug) {
		return nil, domain.ErrInvalidSlug
	}
	name = strings.TrimSpace(name)
	if name == "" || len(name) > maxNameLen {
		return nil, domain.ErrInvalidName
	}

	// Sem upload no MVP: associa metadados de capa gerada (seed = slug). Ver
	// spec 003-generated-cover.
	g := &domain.Game{
		ID:                  uuid.NewString(),
		Slug:                slug,
		Name:                name,
		CreatedAt:           time.Now().UTC(),
		ImageSource:         "generated",
		GeneratedImageSeed:  slug,
		GeneratedImageStyle: defaultImageStyle,
	}
	if err := s.repo.Create(ctx, g); err != nil {
		return nil, fmt.Errorf("game-registry service: register: %w", err)
	}
	return g, nil
}

func (s *service) Rename(ctx context.Context, id, name string) (*domain.Game, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > maxNameLen {
		return nil, domain.ErrInvalidName
	}
	if err := s.repo.UpdateName(ctx, id, name); err != nil {
		return nil, fmt.Errorf("game-registry service: rename: %w", err)
	}
	g, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("game-registry service: rename: reload: %w", err)
	}
	return g, nil
}

// SetFleet atualiza a Fleet Agones do jogo (ADR-0026). Vazio limpa o campo;
// não-vazio precisa casar fleetRE.
func (s *service) SetFleet(ctx context.Context, id, fleet string) (*domain.Game, error) {
	if fleet != "" && !fleetRE.MatchString(fleet) {
		return nil, domain.ErrInvalidFleet
	}
	if err := s.repo.UpdateFleet(ctx, id, fleet); err != nil {
		return nil, fmt.Errorf("game-registry service: set fleet: %w", err)
	}
	g, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("game-registry service: set fleet: reload: %w", err)
	}
	return g, nil
}

// Delete marca o jogo como removido (soft delete, ADR-0025). Sem regra além
// do repasse: a semântica (deletado = inexistente) mora no repository.
func (s *service) Delete(ctx context.Context, id string) error {
	if err := s.repo.SoftDelete(ctx, id); err != nil {
		return fmt.Errorf("game-registry service: delete: %w", err)
	}
	return nil
}

func (s *service) List(ctx context.Context) ([]domain.Game, error) {
	games, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("game-registry service: list: %w", err)
	}
	return games, nil
}

func (s *service) Get(ctx context.Context, id string) (*domain.Game, error) {
	g, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("game-registry service: get: %w", err)
	}
	return g, nil
}
