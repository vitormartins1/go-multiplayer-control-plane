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

// Service agrega as fontes (Prometheus + achievements + identity) para os
// painéis do admin.
type Service struct {
	metrics   domain.MetricsSource
	reader    domain.AchievementsReader
	directory domain.PlayerDirectory
	registry  domain.GameRegistry
}

// New cria o service a partir das Ports de saída.
func New(metrics domain.MetricsSource, reader domain.AchievementsReader, directory domain.PlayerDirectory, registry domain.GameRegistry) *Service {
	return &Service{metrics: metrics, reader: reader, directory: directory, registry: registry}
}

// Players delega ao directory (API admin do identity-service).
func (s *Service) Players(ctx context.Context, limit, offset int) (domain.PlayerPage, error) {
	return s.directory.ListPlayers(ctx, limit, offset)
}

// Games lista os jogos (game-registry).
func (s *Service) Games(ctx context.Context) ([]domain.Game, error) {
	return s.registry.ListGames(ctx)
}

// CreateGame registra um jogo (game-registry).
func (s *Service) CreateGame(ctx context.Context, slug, name string) (*domain.Game, error) {
	return s.registry.CreateGame(ctx, slug, name)
}

// GameCatalog lê o catálogo de conquistas do jogo (achievements).
func (s *Service) GameCatalog(ctx context.Context, gameID string) ([]domain.Achievement, error) {
	return s.reader.CatalogForGame(ctx, gameID)
}

// CreateAchievement cadastra uma conquista no jogo (achievements).
func (s *Service) CreateAchievement(ctx context.Context, gameID string, a domain.Achievement) error {
	return s.reader.CreateAchievement(ctx, gameID, a)
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

// ServiceMetrics agrega saúde + RED + runtime de um serviço (página de domínio).
// Valida o id contra a allowlist (404 via ErrUnknownService) e roda as queries
// em paralelo (errgroup). Campos sem amostra ficam nil (ex.: RED nos gRPC).
func (s *Service) ServiceMetrics(ctx context.Context, id string) (domain.ServiceMetrics, error) {
	if !domain.IsKnownService(id) {
		return domain.ServiceMetrics{}, domain.ErrUnknownService
	}
	sm := domain.ServiceMetrics{ID: id}
	g, gctx := errgroup.WithContext(ctx)
	// Cada goroutine escreve um campo distinto de sm (locais independentes).
	g.Go(func() error {
		v, err := s.scalar(gctx, `up{job="`+id+`"}`)
		sm.Up = floatToBool(v)
		return err
	})
	g.Go(func() (err error) {
		sm.RequestRate, err = s.scalar(gctx, `sum(rate(http_requests_total{service="`+id+`"}[5m]))`)
		return
	})
	g.Go(func() (err error) {
		sm.ErrorRate, err = s.scalar(gctx, `sum(rate(http_requests_total{service="`+id+`",status=~"5.."}[5m])) / sum(rate(http_requests_total{service="`+id+`"}[5m]))`)
		return
	})
	g.Go(func() (err error) {
		sm.LatencyP95, err = s.scalar(gctx, `histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket{service="`+id+`"}[5m])) by (le))`)
		return
	})
	g.Go(func() (err error) { sm.Goroutines, err = s.scalar(gctx, `go_goroutines{job="`+id+`"}`); return })
	g.Go(func() (err error) {
		sm.MemoryBytes, err = s.scalar(gctx, `process_resident_memory_bytes{job="`+id+`"}`)
		return
	})
	if err := g.Wait(); err != nil {
		return domain.ServiceMetrics{}, err
	}
	return sm, nil
}

// floatToBool mapeia o valor de up{} (1/0) para *bool; nil se não houver amostra.
func floatToBool(v *float64) *bool {
	if v == nil {
		return nil
	}
	b := *v == 1
	return &b
}

// PlayerAchievements delega ao reader (achievements do jogador no jogo).
func (s *Service) PlayerAchievements(ctx context.Context, gameID, playerID string) ([]domain.PlayerAchievement, error) {
	return s.reader.PlayerAchievements(ctx, gameID, playerID)
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
