// Package transport traduz HTTP <-> domínio para o admin-bff. Sem lógica de
// negócio (ADR-0002): mapeia a indisponibilidade de cada fonte para o status
// HTTP do painel correspondente (ADR-0016).
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/c130n/game-backend/services/admin-bff/internal/domain"
)

// AdminService é a Port de entrada consumida pelo handler.
type AdminService interface {
	Topology(ctx context.Context) (domain.Topology, error)
	LiveMetrics(ctx context.Context) (domain.LiveMetrics, error)
	Players(ctx context.Context, limit, offset int) (domain.PlayerPage, error)
	Games(ctx context.Context) ([]domain.Game, error)
	CreateGame(ctx context.Context, slug, name string) (*domain.Game, error)
	GameCatalog(ctx context.Context, gameID string) ([]domain.Achievement, error)
	CreateAchievement(ctx context.Context, gameID string, a domain.Achievement) error
	PlayerAchievements(ctx context.Context, gameID, playerID string) ([]domain.PlayerAchievement, error)
	GameLeaderboard(ctx context.Context, gameID string, limit int) ([]domain.LeaderboardEntry, error)
	PlayerData(ctx context.Context, gameID, playerID string) ([]domain.PlayerDataEntry, error)
	CloudSave(ctx context.Context, gameID, playerID string) (domain.CloudSave, error)
	GamePlayers(ctx context.Context, gameID string) ([]domain.GamePlayer, error)
	PlayerGames(ctx context.Context, playerID string) ([]domain.Game, error)
}

// Handler expõe a API HTTP do admin-bff (consumida pelo admin-ui).
type Handler struct {
	svc AdminService
}

// NewHandler cria o handler a partir do service.
func NewHandler(svc AdminService) *Handler {
	return &Handler{svc: svc}
}

// Register monta as rotas da API do BFF no mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/topology", h.topology)
	mux.HandleFunc("GET /api/metrics/live", h.liveMetrics)
	mux.HandleFunc("GET /api/players", h.players)
	mux.HandleFunc("GET /api/games", h.games)
	mux.HandleFunc("POST /api/games", h.createGame)
	mux.HandleFunc("GET /api/games/{gameId}/achievements", h.gameCatalog)
	mux.HandleFunc("POST /api/games/{gameId}/achievements", h.createAchievement)
	mux.HandleFunc("GET /api/games/{gameId}/players/{playerID}/achievements", h.playerAchievements)
	mux.HandleFunc("GET /api/games/{gameId}/leaderboard", h.gameLeaderboard)
	mux.HandleFunc("GET /api/games/{gameId}/players/{playerID}/data", h.playerData)
	mux.HandleFunc("GET /api/games/{gameId}/players/{playerID}/save", h.cloudSave)
	mux.HandleFunc("GET /api/games/{gameId}/players", h.gamePlayers)
	mux.HandleFunc("GET /api/players/{playerID}/games", h.playerGames)
}

// topology nunca falha por causa do Prometheus: o service degrada (nós unknown).
func (h *Handler) topology(w http.ResponseWriter, r *http.Request) {
	topo, err := h.svc.Topology(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, topo)
}

// liveMetrics depende só do Prometheus: fonte fora -> 503.
func (h *Handler) liveMetrics(w http.ResponseWriter, r *http.Request) {
	lm, err := h.svc.LiveMetrics(r.Context())
	if err != nil {
		if errors.Is(err, domain.ErrMetricsUnavailable) {
			writeError(w, http.StatusServiceUnavailable, "metrics_unavailable")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, lm)
}

// players lista os usuários (User base) via identity. limit/offset repassados;
// identity fora -> 502.
func (h *Handler) players(w http.ResponseWriter, r *http.Request) {
	limit := atoiOr(r.URL.Query().Get("limit"), 0)
	offset := atoiOr(r.URL.Query().Get("offset"), 0)

	page, err := h.svc.Players(r.Context(), limit, offset)
	if err != nil {
		if errors.Is(err, domain.ErrIdentityUnavailable) {
			writeError(w, http.StatusBadGateway, "players_unavailable")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func atoiOr(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}

// games lista os jogos (game-registry).
func (h *Handler) games(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.Games(r.Context())
	if err != nil {
		writeUpstream(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// createGame registra um jogo; repassa 400/409 do game-registry.
func (h *Handler) createGame(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Slug string `json:"slug"`
		Name string `json:"name"`
	}
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	g, err := h.svc.CreateGame(r.Context(), req.Slug, req.Name)
	if err != nil {
		writeUpstream(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, g)
}

// gameCatalog lê o catálogo de conquistas do jogo (achievements).
func (h *Handler) gameCatalog(w http.ResponseWriter, r *http.Request) {
	cat, err := h.svc.GameCatalog(r.Context(), r.PathValue("gameId"))
	if err != nil {
		writeUpstream(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cat)
}

// createAchievement cadastra uma conquista no jogo; repassa 400/409.
func (h *Handler) createAchievement(w http.ResponseWriter, r *http.Request) {
	var a domain.Achievement
	if err := decode(r, &a); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	if err := h.svc.CreateAchievement(r.Context(), r.PathValue("gameId"), a); err != nil {
		writeUpstream(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, a)
}

// playerAchievements: achievements do jogador no jogo (achievements).
func (h *Handler) playerAchievements(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.PlayerAchievements(r.Context(), r.PathValue("gameId"), r.PathValue("playerID"))
	if err != nil {
		writeUpstream(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// gameLeaderboard lê o top-N do jogo (leaderboard).
func (h *Handler) gameLeaderboard(w http.ResponseWriter, r *http.Request) {
	limit := atoiOr(r.URL.Query().Get("limit"), 0)
	entries, err := h.svc.GameLeaderboard(r.Context(), r.PathValue("gameId"), limit)
	if err != nil {
		writeUpstream(w, err)
		return
	}
	if entries == nil {
		entries = []domain.LeaderboardEntry{}
	}
	writeJSON(w, http.StatusOK, entries)
}

// gamePlayers: a player base do jogo (presença, ADR-0019).
func (h *Handler) gamePlayers(w http.ResponseWriter, r *http.Request) {
	players, err := h.svc.GamePlayers(r.Context(), r.PathValue("gameId"))
	if err != nil {
		writeUpstream(w, err)
		return
	}
	if players == nil {
		players = []domain.GamePlayer{}
	}
	writeJSON(w, http.StatusOK, players)
}

// playerGames: os jogos em que o player está presente (consulta reversa).
func (h *Handler) playerGames(w http.ResponseWriter, r *http.Request) {
	games, err := h.svc.PlayerGames(r.Context(), r.PathValue("playerID"))
	if err != nil {
		writeUpstream(w, err)
		return
	}
	if games == nil {
		games = []domain.Game{}
	}
	writeJSON(w, http.StatusOK, games)
}

// playerData: dado de gameplay do jogador no jogo (player-data).
func (h *Handler) playerData(w http.ResponseWriter, r *http.Request) {
	entries, err := h.svc.PlayerData(r.Context(), r.PathValue("gameId"), r.PathValue("playerID"))
	if err != nil {
		writeUpstream(w, err)
		return
	}
	if entries == nil {
		entries = []domain.PlayerDataEntry{}
	}
	writeJSON(w, http.StatusOK, entries)
}

// cloudSave: save do jogador no jogo (cloud-save). 404 (sem save) é repassado
// via UpstreamError pelo writeUpstream.
func (h *Handler) cloudSave(w http.ResponseWriter, r *http.Request) {
	save, err := h.svc.CloudSave(r.Context(), r.PathValue("gameId"), r.PathValue("playerID"))
	if err != nil {
		writeUpstream(w, err)
		return
	}
	writeJSON(w, http.StatusOK, save)
}

// decode lê o corpo JSON rejeitando campos desconhecidos.
func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// writeUpstream traduz erros de upstream: UpstreamError repassa o status (4xx);
// indisponibilidade (game-registry/achievements) -> 502; resto -> 500.
func writeUpstream(w http.ResponseWriter, err error) {
	var ue *domain.UpstreamError
	switch {
	case errors.As(err, &ue):
		code := ue.Code
		if code == "" {
			code = "upstream_error"
		}
		writeError(w, ue.Status, code)
	case errors.Is(err, domain.ErrGameRegistryUnavailable):
		writeError(w, http.StatusBadGateway, "game_registry_unavailable")
	case errors.Is(err, domain.ErrAchievementsUnavailable):
		writeError(w, http.StatusBadGateway, "achievements_unavailable")
	case errors.Is(err, domain.ErrLeaderboardUnavailable):
		writeError(w, http.StatusBadGateway, "leaderboard_unavailable")
	case errors.Is(err, domain.ErrPlayerDataUnavailable):
		writeError(w, http.StatusBadGateway, "player_data_unavailable")
	case errors.Is(err, domain.ErrCloudSaveUnavailable):
		writeError(w, http.StatusBadGateway, "cloud_save_unavailable")
	default:
		writeError(w, http.StatusInternalServerError, "internal")
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}
