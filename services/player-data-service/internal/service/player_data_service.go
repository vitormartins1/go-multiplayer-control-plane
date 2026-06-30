// Package service contém a regra de negócio do player-data-service.
// Só depende das interfaces de domain (ADR-0002).
package service

import (
	"context"
	"fmt"
	"regexp"

	"github.com/c130n/game-backend/services/player-data-service/internal/domain"
)

var (
	namespaceRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
	keyRe       = regexp.MustCompile(`^[a-zA-Z0-9_.-]{1,128}$`)
)

// garante em tempo de compilação que satisfazemos a Port de entrada.
var _ domain.PlayerDataService = (*Service)(nil)

// Service implementa o acesso a dados de gameplay por jogador.
type Service struct {
	repo domain.PlayerDataRepository
}

// New cria o service a partir do repositório.
func New(repo domain.PlayerDataRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Get(ctx context.Context, gameID, playerID, namespace, key string) ([]byte, error) {
	if err := validateKey(namespace, key); err != nil {
		return nil, err
	}
	v, err := s.repo.Get(ctx, gameID, playerID, namespace, key)
	if err != nil {
		return nil, fmt.Errorf("player-data service: get: %w", err)
	}
	return v, nil
}

func (s *Service) GetNamespace(ctx context.Context, gameID, playerID, namespace string) (map[string][]byte, error) {
	if !namespaceRe.MatchString(namespace) {
		return nil, domain.ErrInvalidKey
	}
	m, err := s.repo.GetNamespace(ctx, gameID, playerID, namespace)
	if err != nil {
		return nil, fmt.Errorf("player-data service: get namespace: %w", err)
	}
	return m, nil
}

func (s *Service) Put(ctx context.Context, gameID, playerID, namespace, key string, value []byte) error {
	if err := validateKey(namespace, key); err != nil {
		return err
	}
	if err := s.repo.Put(ctx, gameID, playerID, namespace, key, value); err != nil {
		return fmt.Errorf("player-data service: put: %w", err)
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, gameID, playerID, namespace, key string) error {
	if err := validateKey(namespace, key); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, gameID, playerID, namespace, key); err != nil {
		return fmt.Errorf("player-data service: delete: %w", err)
	}
	return nil
}

// ListForPlayer devolve todo o dado de um jogador num jogo (admin read).
func (s *Service) ListForPlayer(ctx context.Context, gameID, playerID string) ([]domain.Entry, error) {
	entries, err := s.repo.ListForPlayer(ctx, gameID, playerID)
	if err != nil {
		return nil, fmt.Errorf("player-data service: list for player: %w", err)
	}
	return entries, nil
}

// validateKey rejeita namespace/key fora do padrão (ErrInvalidKey).
func validateKey(namespace, key string) error {
	if !namespaceRe.MatchString(namespace) || !keyRe.MatchString(key) {
		return domain.ErrInvalidKey
	}
	return nil
}
