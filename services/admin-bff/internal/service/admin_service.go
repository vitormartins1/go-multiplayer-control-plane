// Package service orquestra o admin-bff: monta a topologia e os contadores ao
// vivo a partir do Prometheus e delega os achievements ao reader. Só depende
// das Ports de domain (ADR-0002). É read-only e stateless (ADR-0016).
package service

import (
	"context"
	"errors"

	"golang.org/x/sync/errgroup"

	"github.com/vitormartins1/go-game-backend/services/admin-bff/internal/domain"
)

// Service agrega as fontes (Prometheus + serviços de jogo) para os painéis do
// admin.
type Service struct {
	metrics     domain.MetricsSource
	reader      domain.AchievementsReader
	directory   domain.PlayerDirectory
	registry    domain.GameRegistry
	leaderboard domain.LeaderboardReader
	playerData  domain.PlayerDataReader
	cloudSave   domain.CloudSaveReader
	sessions    domain.SessionsReader
}

// Deps reúne as Ports de saída do service. Struct (em vez de args posicionais)
// porque já são 7 dependências: campos nomeados deixam o wire-up e os testes
// legíveis e evitam erro de ordem.
type Deps struct {
	Metrics     domain.MetricsSource
	Reader      domain.AchievementsReader
	Directory   domain.PlayerDirectory
	Registry    domain.GameRegistry
	Leaderboard domain.LeaderboardReader
	PlayerData  domain.PlayerDataReader
	CloudSave   domain.CloudSaveReader
	Sessions    domain.SessionsReader
}

// New cria o service a partir das Ports de saída.
func New(d Deps) *Service {
	return &Service{
		metrics:     d.Metrics,
		reader:      d.Reader,
		directory:   d.Directory,
		registry:    d.Registry,
		leaderboard: d.Leaderboard,
		playerData:  d.PlayerData,
		cloudSave:   d.CloudSave,
		sessions:    d.Sessions,
	}
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

// UpdateAchievement edita uma conquista do jogo (achievements).
func (s *Service) UpdateAchievement(ctx context.Context, gameID, id string, a domain.Achievement) error {
	return s.reader.UpdateAchievement(ctx, gameID, id, a)
}

// DeleteAchievement remove uma conquista do jogo (achievements).
func (s *Service) DeleteAchievement(ctx context.Context, gameID, id string) error {
	return s.reader.DeleteAchievement(ctx, gameID, id)
}

// GameLeaderboard lê o top-N do jogo (leaderboard).
func (s *Service) GameLeaderboard(ctx context.Context, gameID string, limit int) ([]domain.LeaderboardEntry, error) {
	return s.leaderboard.TopForGame(ctx, gameID, limit)
}

// GamePlayers lê a player base do jogo (presença, ADR-0019).
func (s *Service) GamePlayers(ctx context.Context, gameID string) ([]domain.GamePlayer, error) {
	return s.registry.PlayersOf(ctx, gameID)
}

// PlayerGames lê os jogos em que o player está presente (consulta reversa).
func (s *Service) PlayerGames(ctx context.Context, playerID string) ([]domain.Game, error) {
	return s.registry.GamesOfPlayer(ctx, playerID)
}

// GameSessions lê as sessões do jogo (session-service) e enriquece os players
// com o handle do identity — best-effort: identity fora não derruba a resposta
// (handle vazio, id sempre presente). Session fora -> ErrSessionsUnavailable.
func (s *Service) GameSessions(ctx context.Context, gameID, state string) (domain.GameSessions, error) {
	sessions, err := s.sessions.SessionsByGame(ctx, gameID, state)
	if err != nil {
		return domain.GameSessions{}, err
	}

	handles := s.handlesByID(ctx)
	enrich := func(list []domain.GameSession) {
		for i := range list {
			for j := range list[i].Players {
				list[i].Players[j].Handle = handles[list[i].Players[j].ID]
			}
		}
	}
	enrich(sessions.Active)
	enrich(sessions.Closed)
	return sessions, nil
}

// Rooms lê as sessões de TODOS os jogos (board global do console, 008) e
// enriquece cada uma com o handle dos players (identity) e o nome do jogo
// (game-registry). Best-effort: fontes fora -> campos vazios, não derruba.
func (s *Service) Rooms(ctx context.Context, state string) (domain.GameSessions, error) {
	sessions, err := s.sessions.AllSessions(ctx, state)
	if err != nil {
		return domain.GameSessions{}, err
	}

	handles := s.handlesByID(ctx)
	names := s.gameNamesByID(ctx)
	enrich := func(list []domain.GameSession) {
		for i := range list {
			list[i].GameName = names[list[i].GameID]
			for j := range list[i].Players {
				list[i].Players[j].Handle = handles[list[i].Players[j].ID]
			}
		}
	}
	enrich(sessions.Active)
	enrich(sessions.Closed)
	return sessions, nil
}

// CloseRoom encerra uma sessão ativa por ação do operador (force-close, 008).
func (s *Service) CloseRoom(ctx context.Context, matchID string) (domain.GameSession, error) {
	return s.sessions.CloseSession(ctx, matchID)
}

// FanoutHealth agrega os sinais do fan-out durável (ADR-0020) via PromQL, em
// paralelo. Prometheus fora -> ErrMetricsUnavailable (transport -> 503); métrica
// ausente -> campo nil.
func (s *Service) FanoutHealth(ctx context.Context) (domain.FanoutHealth, error) {
	var fh domain.FanoutHealth
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) { fh.PublishOK, err = s.scalar(ctx, `sum(fanout_publish_total{result="ok"})`); return })
	g.Go(func() (err error) { fh.PublishError, err = s.scalar(ctx, `sum(fanout_publish_total{result="error"})`); return })
	g.Go(func() (err error) { fh.ConsumeOK, err = s.scalar(ctx, `sum(fanout_consume_total{result="ok"})`); return })
	g.Go(func() (err error) { fh.ConsumeDup, err = s.scalar(ctx, `sum(fanout_consume_total{result="dup"})`); return })
	g.Go(func() (err error) { fh.ConsumeError, err = s.scalar(ctx, `sum(fanout_consume_total{result="error"})`); return })
	if err := g.Wait(); err != nil {
		return domain.FanoutHealth{}, err
	}
	return fh, nil
}

// gameNamesByID monta o mapa gameID -> nome (via registry.ListGames). Best-effort:
// registry fora -> mapa vazio (o board mostra a sala sem o nome do jogo).
func (s *Service) gameNamesByID(ctx context.Context) map[string]string {
	names := make(map[string]string)
	games, err := s.registry.ListGames(ctx)
	if err != nil {
		return names
	}
	for _, g := range games {
		names[g.ID] = g.Name
	}
	return names
}

// handlesByID monta o mapa playerID -> handle paginando o ListPlayers do
// identity (não há lookup por id — validado na spec). Bounded a poucas páginas:
// suficiente para a base de dev; falha em qualquer página -> mapa parcial/vazio
// (best-effort de verdade).
func (s *Service) handlesByID(ctx context.Context) map[string]string {
	const pageSize = 200 // maxLimit do identity
	const maxPages = 5

	handles := make(map[string]string)
	for page := 0; page < maxPages; page++ {
		result, err := s.directory.ListPlayers(ctx, pageSize, page*pageSize)
		if err != nil {
			return handles
		}
		for _, p := range result.Players {
			handles[p.ID] = p.Handle
		}
		if (page+1)*pageSize >= result.Total {
			break
		}
	}
	return handles
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

// PlayerAchievements delega ao reader (achievements do jogador no jogo).
func (s *Service) PlayerAchievements(ctx context.Context, gameID, playerID string) ([]domain.PlayerAchievement, error) {
	return s.reader.PlayerAchievements(ctx, gameID, playerID)
}

// PlayerData lê o dado de gameplay do jogador no jogo (player-data).
func (s *Service) PlayerData(ctx context.Context, gameID, playerID string) ([]domain.PlayerDataEntry, error) {
	return s.playerData.ForPlayer(ctx, gameID, playerID)
}

// CloudSave lê o save do jogador no jogo (cloud-save). 404 (sem save) vira
// UpstreamError, repassado pelo transport.
func (s *Service) CloudSave(ctx context.Context, gameID, playerID string) (domain.CloudSave, error) {
	return s.cloudSave.ForPlayer(ctx, gameID, playerID)
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
