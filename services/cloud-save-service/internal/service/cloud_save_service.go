// Package service contém a regra de negócio do cloud-save-service.
// Só depende das interfaces de domain (ADR-0002).
package service

import (
	"context"
	"fmt"

	"github.com/c130n/game-backend/services/cloud-save-service/internal/domain"
)

// garante em tempo de compilação que satisfazemos a Port de entrada.
var _ domain.CloudSaveService = (*Service)(nil)

// Service orquestra o save/load do estado do jogo.
type Service struct {
	store domain.SaveStore
}

// New cria o service a partir do store de saves.
func New(store domain.SaveStore) *Service {
	return &Service{store: store}
}

func (s *Service) Save(ctx context.Context, gameID, playerID string, snapshot domain.Snapshot) error {
	if err := s.store.Put(ctx, gameID, playerID, snapshot); err != nil {
		return fmt.Errorf("cloud-save service: save: %w", err)
	}
	return nil
}

func (s *Service) Load(ctx context.Context, gameID, playerID string) (domain.Snapshot, error) {
	snap, err := s.store.Get(ctx, gameID, playerID)
	if err != nil {
		return domain.Snapshot{}, fmt.Errorf("cloud-save service: load: %w", err)
	}
	return snap, nil
}

// LoadFor é o read administrativo do save de um jogador num jogo. Mesma
// operação do Load (escopo por jogo); o nome separa a intenção admin no
// transport (jogo/jogador vêm do path, não do JWT).
func (s *Service) LoadFor(ctx context.Context, gameID, playerID string) (domain.Snapshot, error) {
	return s.Load(ctx, gameID, playerID)
}
