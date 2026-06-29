// Package service contém o núcleo do session-service: orquestra a criação de
// sessões (aloca server → emite connect tokens → registra) e o encerramento
// (publica resultado → libera server → encerra). Só depende das interfaces de
// domain (ADR-0002). Estado em memória, protegido por mutex.
package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"

	"github.com/c130n/game-backend/services/session-service/internal/domain"
)

var _ domain.SessionService = (*Service)(nil)

// Service orquestra sessões a partir das Ports de saída (allocator, issuer,
// publisher) e mantém as sessões ativas em memória.
type Service struct {
	allocator domain.ServerAllocator
	issuer    domain.ConnectTokenIssuer
	publisher domain.ResultPublisher

	mu       sync.Mutex
	sessions map[string]*domain.Session // matchID -> sessão ativa
}

// New cria o service a partir das Ports de saída.
func New(allocator domain.ServerAllocator, issuer domain.ConnectTokenIssuer, publisher domain.ResultPublisher) *Service {
	return &Service{
		allocator: allocator,
		issuer:    issuer,
		publisher: publisher,
		sessions:  make(map[string]*domain.Session),
	}
}

// Create aloca um game server, emite um connect token por jogador e registra a
// sessão. Se a emissão falhar, o server alocado é devolvido ao pool.
func (s *Service) Create(ctx context.Context, matchID string, players []string) (*domain.Session, []domain.ConnectGrant, error) {
	server, err := s.allocator.Allocate(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("session service: create: allocate: %w", err)
	}

	grants := make([]domain.ConnectGrant, 0, len(players))
	for _, playerID := range players {
		tok, err := s.issuer.Issue(playerID, matchID, server.Address)
		if err != nil {
			// rollback: não deixa o server preso se não conseguimos cunhar os tokens.
			_ = s.allocator.Release(ctx, server.ID)
			return nil, nil, fmt.Errorf("session service: create: issue token: %w", err)
		}
		grants = append(grants, domain.ConnectGrant{PlayerID: playerID, Token: tok})
	}

	sess := &domain.Session{
		ID:            uuid.NewString(),
		MatchID:       matchID,
		Players:       players,
		ServerID:      server.ID,
		ServerAddress: server.Address,
	}

	s.mu.Lock()
	s.sessions[matchID] = sess
	s.mu.Unlock()

	return sess, grants, nil
}

// ReportResult dispara o fan-out do resultado, libera o server e encerra a
// sessão. ErrSessionNotFound se não houver sessão para o match.
func (s *Service) ReportResult(ctx context.Context, matchID string, results []domain.PlayerResult) error {
	s.mu.Lock()
	sess, ok := s.sessions[matchID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("session service: report result: %w", domain.ErrSessionNotFound)
	}
	delete(s.sessions, matchID)
	s.mu.Unlock()

	if err := s.publisher.Publish(ctx, matchID, results); err != nil {
		return fmt.Errorf("session service: report result: publish: %w", err)
	}
	if err := s.allocator.Release(ctx, sess.ServerID); err != nil {
		return fmt.Errorf("session service: report result: release: %w", err)
	}
	return nil
}

// ActiveSessions devolve o número de sessões ativas (métrica).
func (s *Service) ActiveSessions() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return float64(len(s.sessions))
}
