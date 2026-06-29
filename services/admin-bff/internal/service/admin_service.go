// Package service orquestra o admin-bff: monta a topologia e os contadores ao
// vivo a partir do Prometheus e delega os achievements ao reader. Só depende
// das Ports de domain (ADR-0002). É read-only e stateless (ADR-0016).
package service

import (
	"context"
	"errors"

	"golang.org/x/sync/errgroup"

	"github.com/c130n/game-backend/services/admin-bff/internal/domain"
)

// Service agrega as fontes (Prometheus + achievements) para os painéis do admin.
type Service struct {
	metrics domain.MetricsSource
	reader  domain.AchievementsReader
}

// New cria o service a partir das Ports de saída.
func New(metrics domain.MetricsSource, reader domain.AchievementsReader) *Service {
	return &Service{metrics: metrics, reader: reader}
}

// Topology monta o mapa de serviços com o status up/down vindo de `up{}`. Se o
// Prometheus estiver fora, degrada: nós com Up nil e SourceAvailable=false.
func (s *Service) Topology(ctx context.Context) (domain.Topology, error) {
	nodes := make([]domain.ServiceNode, len(domain.KnownServices))
	copy(nodes, domain.KnownServices)

	samples, err := s.metrics.Query(ctx, "up")
	if err != nil {
		if errors.Is(err, domain.ErrMetricsUnavailable) {
			return domain.Topology{Nodes: nodes, Edges: domain.KnownEdges, SourceAvailable: false}, nil
		}
		return domain.Topology{}, err
	}

	upByJob := make(map[string]bool, len(samples))
	for _, sm := range samples {
		upByJob[sm.Labels["job"]] = sm.Value == 1
	}
	for i := range nodes {
		up := upByJob[nodes[i].ID] // ausente -> false (a fonte respondeu; o alvo é que não está up)
		nodes[i].Up = &up
	}
	return domain.Topology{Nodes: nodes, Edges: domain.KnownEdges, SourceAvailable: true}, nil
}

// LiveMetrics consulta os contadores ao vivo em paralelo (errgroup). Se o
// Prometheus estiver fora, devolve ErrMetricsUnavailable (transport -> 503).
func (s *Service) LiveMetrics(ctx context.Context) (domain.LiveMetrics, error) {
	var lm domain.LiveMetrics
	g, ctx := errgroup.WithContext(ctx)
	// Cada goroutine escreve um campo distinto de lm (locais de memória
	// independentes) — seguro sob -race.
	g.Go(func() (err error) { lm.QueueSize, err = s.scalar(ctx, "matchmaking_queue_size"); return })
	g.Go(func() (err error) { lm.ActiveSessions, err = s.scalar(ctx, "session_active_sessions"); return })
	g.Go(func() (err error) { lm.MatchesTotal, err = s.scalar(ctx, "matchmaking_matches_total"); return })
	g.Go(func() (err error) { lm.MatchesFailed, err = s.scalar(ctx, "matchmaking_matches_failed_total"); return })
	if err := g.Wait(); err != nil {
		return domain.LiveMetrics{}, err
	}
	return lm, nil
}

// PlayerAchievements delega ao reader (API admin do achievements-service).
func (s *Service) PlayerAchievements(ctx context.Context, playerID string) ([]domain.PlayerAchievement, error) {
	return s.reader.ForPlayer(ctx, playerID)
}

// scalar roda uma instant query e devolve o primeiro valor (nil se a métrica
// não existir/não foi exposta ainda).
func (s *Service) scalar(ctx context.Context, promQL string) (*float64, error) {
	samples, err := s.metrics.Query(ctx, promQL)
	if err != nil {
		return nil, err
	}
	if len(samples) == 0 {
		return nil, nil
	}
	v := samples[0].Value
	return &v, nil
}
