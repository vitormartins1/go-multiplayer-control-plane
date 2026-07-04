// Package service contém o núcleo do session-service: orquestra a criação de
// sessões (aloca server → emite connect tokens → registra) e o encerramento
// (publica resultado → libera server → encerra). Só depende das interfaces de
// domain (ADR-0002). Estado em memória, protegido por mutex.
package service

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/vitormartins1/go-game-backend/services/session-service/internal/domain"
)

var _ domain.SessionService = (*Service)(nil)

// Service orquestra sessões a partir das Ports de saída (allocator, issuer,
// publisher) e mantém as sessões ativas em memória.
type Service struct {
	allocator domain.ServerAllocator
	issuer    domain.ConnectTokenIssuer
	publisher domain.ResultPublisher
	now       func() time.Time // injetável: o sweeper de TTL precisa ser testável

	mu       sync.Mutex
	sessions map[string]*domain.Session // matchID -> sessão ativa
	closed   map[domain.CloseReason]float64

	// histórico bounded de fechadas (spec 005): ring em memória, best-effort
	// de dev — some no restart, persistência é evolução.
	history      []*domain.Session
	historyLimit int
}

// Option configura o Service na construção (só testes precisam disso).
type Option func(*Service)

// WithClock troca o relógio (default time.Now) — o sweeper decide TTL por ele.
func WithClock(now func() time.Time) Option {
	return func(s *Service) { s.now = now }
}

// WithHistoryLimit define o tamanho do histórico de fechadas (default 200).
func WithHistoryLimit(n int) Option {
	return func(s *Service) {
		if n > 0 {
			s.historyLimit = n
		}
	}
}

// New cria o service a partir das Ports de saída.
func New(allocator domain.ServerAllocator, issuer domain.ConnectTokenIssuer, publisher domain.ResultPublisher, opts ...Option) *Service {
	s := &Service{
		allocator:    allocator,
		issuer:       issuer,
		publisher:    publisher,
		now:          time.Now,
		sessions:     make(map[string]*domain.Session),
		closed:       make(map[domain.CloseReason]float64),
		historyLimit: 200,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Create aloca um game server, emite um connect token por jogador e registra a
// sessão (OPEN). Se a emissão falhar, o server alocado é devolvido ao pool.
func (s *Service) Create(ctx context.Context, matchID, gameID string, players []string) (*domain.Session, []domain.ConnectGrant, error) {
	server, err := s.allocator.Allocate(ctx, matchID, gameID)
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
		GameID:        gameID,
		Players:       players,
		ServerID:      server.ID,
		ServerAddress: server.Address,
		State:         domain.SessionOpen,
		OpenedAt:      s.now(),
	}

	s.mu.Lock()
	s.sessions[matchID] = sess
	s.mu.Unlock()

	return sess, grants, nil
}

// ReportResult publica o resultado no fan-out e, só então, libera o server e
// encerra a sessão (CLOSED/result). ErrSessionNotFound se não houver sessão para
// o match.
//
// Ordem invertida vs. specs 002/003 (ADR-0020): publica ANTES de fechar. Se a
// publicação falhar, retorna erro e NÃO encerra a sessão — o Dedicated Server
// refaz o ReportResult (retry, Fase 6 do data plane) e o TTL sweeper cobre a
// sessão órfã. O resultado não se perde silenciosamente.
func (s *Service) ReportResult(ctx context.Context, matchID string, results []domain.PlayerResult) error {
	s.mu.Lock()
	sess, ok := s.sessions[matchID]
	gameID := ""
	if ok {
		gameID = sess.GameID
	}
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("session service: report result: %w", domain.ErrSessionNotFound)
	}

	if err := s.publisher.Publish(ctx, gameID, matchID, results); err != nil {
		return fmt.Errorf("session service: report result: publish: %w", err)
	}

	if _, err := s.closeSession(ctx, matchID, domain.CloseResult); err != nil {
		return fmt.Errorf("session service: report result: %w", err)
	}
	return nil
}

// closeSession é o fechamento unificado (spec 004): marca CLOSED com motivo,
// tira do mapa de ativas e libera o server — usado por ReportResult (result) e
// pelo sweeper (ttl). Libera o server PRIMEIRO em relação a qualquer fan-out:
// o fim da partida não pode depender de destino externo.
func (s *Service) closeSession(ctx context.Context, matchID string, reason domain.CloseReason) (*domain.Session, error) {
	s.mu.Lock()
	sess, ok := s.sessions[matchID]
	if !ok {
		s.mu.Unlock()
		return nil, domain.ErrSessionNotFound
	}
	delete(s.sessions, matchID)
	sess.State = domain.SessionClosed
	sess.ClosedAt = s.now()
	sess.CloseReason = reason
	s.closed[reason]++
	s.history = append(s.history, sess)
	if len(s.history) > s.historyLimit {
		s.history = s.history[len(s.history)-s.historyLimit:]
	}
	s.mu.Unlock()

	if err := s.allocator.Release(ctx, sess.ServerID); err != nil {
		return sess, fmt.Errorf("release: %w", err)
	}
	return sess, nil
}

// Sweep fecha (CLOSED/ttl) as sessões abertas há mais que ttl e devolve
// quantas fechou. Erros de release não interrompem a varredura.
func (s *Service) Sweep(ctx context.Context, ttl time.Duration) int {
	cutoff := s.now().Add(-ttl)
	s.mu.Lock()
	var expired []string
	for matchID, sess := range s.sessions {
		if sess.OpenedAt.Before(cutoff) {
			expired = append(expired, matchID)
		}
	}
	s.mu.Unlock()

	for _, matchID := range expired {
		// ErrSessionNotFound = fechada por result entre a varredura e o close; ok.
		_, _ = s.closeSession(ctx, matchID, domain.CloseTTL)
	}
	return len(expired)
}

// RunSweeper roda Sweep a cada interval até o ctx encerrar (goroutine no main,
// shutdown limpo via context — mesmo padrão do Run do matchmaking).
func (s *Service) RunSweeper(ctx context.Context, ttl, interval time.Duration, log *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n := s.Sweep(ctx, ttl); n > 0 {
				log.Info("sweeper: sessões expiradas fechadas", "count", n, "ttl", ttl)
			}
		}
	}
}

// SessionsByGame devolve as sessões do jogo para a superfície admin (spec 005):
// abertas (do mapa de ativas) e fechadas (do histórico bounded), ambas em ordem
// da mais recente para a mais antiga. Devolve cópias — o chamador não enxerga
// o estado interno.
func (s *Service) SessionsByGame(gameID string) (active, closed []domain.Session) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, sess := range s.sessions {
		if sess.GameID == gameID {
			active = append(active, *sess)
		}
	}
	sort.Slice(active, func(i, j int) bool { return active[i].OpenedAt.After(active[j].OpenedAt) })

	// o ring guarda em ordem de fechamento; percorrer de trás = mais recente primeiro.
	for i := len(s.history) - 1; i >= 0; i-- {
		if s.history[i].GameID == gameID {
			closed = append(closed, *s.history[i])
		}
	}
	return active, closed
}

// AllSessions devolve as sessões de TODOS os jogos para o board global do
// console (spec 007): ativas (do mapa) e fechadas (do histórico bounded), da mais
// recente para a mais antiga. Devolve cópias. Irmão global do SessionsByGame.
func (s *Service) AllSessions() (active, closed []domain.Session) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, sess := range s.sessions {
		active = append(active, *sess)
	}
	sort.Slice(active, func(i, j int) bool { return active[i].OpenedAt.After(active[j].OpenedAt) })

	for i := len(s.history) - 1; i >= 0; i-- {
		closed = append(closed, *s.history[i])
	}
	return active, closed
}

// CloseByAdmin encerra uma sessão ativa por ação do operador (CLOSED/admin):
// reusa o fechamento unificado (libera o server, entra no histórico). Devolve
// ErrSessionNotFound se não houver sessão ativa para o match (idempotente do
// ponto de vista do operador). Spec 007.
func (s *Service) CloseByAdmin(ctx context.Context, matchID string) (*domain.Session, error) {
	sess, err := s.closeSession(ctx, matchID, domain.CloseAdmin)
	if err != nil {
		return nil, fmt.Errorf("session service: close by admin: %w", err)
	}
	return sess, nil
}

// ActiveSessions devolve o número de sessões ativas (métrica).
func (s *Service) ActiveSessions() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return float64(len(s.sessions))
}

// ClosedTotal devolve o total de sessões fechadas pelo motivo (métrica
// sessions_closed_total{reason} — registrada como CounterFunc no main).
func (s *Service) ClosedTotal(reason domain.CloseReason) float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed[reason]
}
