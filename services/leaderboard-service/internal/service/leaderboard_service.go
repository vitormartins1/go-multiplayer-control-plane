// Package service contém a regra de negócio do leaderboard-service.
// Só depende das interfaces de domain (ADR-0002).
package service

import (
	"context"
	"fmt"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/leaderboard-service/internal/domain"
)

const (
	defaultLimit  = 10
	maxLimit      = 100
	defaultAround = 5
	maxAround     = 50
)

// garante em tempo de compilação que satisfazemos a Port de entrada.
var _ domain.LeaderboardService = (*Service)(nil)

// Service orquestra submissão e leitura do ranking.
type Service struct {
	store domain.LeaderboardStore
}

// New cria o service a partir do store.
func New(store domain.LeaderboardStore) *Service {
	return &Service{store: store}
}

// Submit aplica o highscore e devolve a posição/score do jogador no board.
func (s *Service) Submit(ctx context.Context, gameID, board, playerID string, score int64) (domain.Entry, error) {
	if !domain.ValidBoard(board) {
		return domain.Entry{}, domain.ErrInvalidBoard
	}
	if err := s.store.Submit(ctx, gameID, board, playerID, score); err != nil {
		return domain.Entry{}, fmt.Errorf("leaderboard service: submit: %w", err)
	}
	// A entry devolvida é a do all-time (contrato pré-janelas preservado).
	rank0, best, err := s.store.Rank(ctx, gameID, board, domain.WindowAllTime, playerID)
	if err != nil {
		return domain.Entry{}, fmt.Errorf("leaderboard service: rank: %w", err)
	}
	return domain.Entry{Rank: rank0 + 1, PlayerID: playerID, Score: best}, nil
}

// ForceSubmit substitui o score independente do valor atual (admin override).
func (s *Service) ForceSubmit(ctx context.Context, gameID, board, playerID string, score int64) (domain.Entry, error) {
	if !domain.ValidBoard(board) {
		return domain.Entry{}, domain.ErrInvalidBoard
	}
	if err := s.store.ForceScore(ctx, gameID, board, playerID, score); err != nil {
		return domain.Entry{}, fmt.Errorf("leaderboard service: force submit: %w", err)
	}
	rank0, best, err := s.store.Rank(ctx, gameID, board, domain.WindowAllTime, playerID)
	if err != nil {
		return domain.Entry{}, fmt.Errorf("leaderboard service: rank after force: %w", err)
	}
	return domain.Entry{Rank: rank0 + 1, PlayerID: playerID, Score: best}, nil
}

// ApplyCumulative soma as parcelas nos boards cumulativos de uma partida (vitórias,
// totais de stat) de forma idempotente por matchID. Parcelas com board inválido são
// descartadas (não derrubam o processamento); sem parcelas válidas é no-op.
func (s *Service) ApplyCumulative(ctx context.Context, gameID, matchID string, incrs []domain.Increment) (bool, error) {
	valid := incrs[:0:0] // nova slice, não reusa o array do caller
	for _, inc := range incrs {
		if domain.ValidBoard(inc.Board) {
			valid = append(valid, inc)
		}
	}
	dup, err := s.store.ApplyCumulative(ctx, gameID, matchID, valid)
	if err != nil {
		return false, fmt.Errorf("leaderboard service: apply cumulative: %w", err)
	}
	return dup, nil
}

// Top devolve a página [offset, offset+limit) da janela pedida (spec 007).
func (s *Service) Top(ctx context.Context, gameID, board, window string, limit, offset int) ([]domain.Entry, error) {
	if !domain.ValidBoard(board) {
		return nil, domain.ErrInvalidBoard
	}
	window, err := resolveWindow(window)
	if err != nil {
		return nil, err
	}
	limit = clampLimit(limit)
	start := int64(clampOffset(offset))
	entries, err := s.store.Range(ctx, gameID, board, window, start, start+int64(limit)-1)
	if err != nil {
		return nil, fmt.Errorf("leaderboard service: top: %w", err)
	}
	return entries, nil
}

func (s *Service) Around(ctx context.Context, gameID, board, window, playerID string, k int) ([]domain.Entry, error) {
	if !domain.ValidBoard(board) {
		return nil, domain.ErrInvalidBoard
	}
	window, err := resolveWindow(window)
	if err != nil {
		return nil, err
	}
	k = clampAround(k)
	rank0, _, err := s.store.Rank(ctx, gameID, board, window, playerID)
	if err != nil {
		return nil, fmt.Errorf("leaderboard service: around: %w", err) // ErrNotRanked propaga
	}
	start := rank0 - int64(k)
	if start < 0 {
		start = 0
	}
	stop := rank0 + int64(k)
	entries, err := s.store.Range(ctx, gameID, board, window, start, stop)
	if err != nil {
		return nil, fmt.Errorf("leaderboard service: around range: %w", err)
	}
	return entries, nil
}

// resolveWindow aplica o default (vazia -> all-time, robustez para chamadores
// internos) e valida o nome.
func resolveWindow(window string) (string, error) {
	if window == "" {
		return domain.WindowAllTime, nil
	}
	if !domain.ValidWindow(window) {
		return "", domain.ErrInvalidWindow
	}
	return window, nil
}

func clampLimit(n int) int {
	switch {
	case n < 1:
		return defaultLimit
	case n > maxLimit:
		return maxLimit
	default:
		return n
	}
}

func clampOffset(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

func clampAround(k int) int {
	switch {
	case k < 0:
		return defaultAround
	case k > maxAround:
		return maxAround
	default:
		return k
	}
}
