// Package transport traduz HTTP <-> domínio para o admin-bff. Sem lógica de
// negócio (ADR-0002): mapeia a indisponibilidade de cada fonte para o status
// HTTP do painel correspondente (ADR-0016).
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/admin-bff/internal/domain"
	"github.com/vitormartins1/go-multiplayer-control-plane/shared/httpkit"
)

// AdminService é a Port de entrada consumida pelo handler.
type AdminService interface {
	Topology(ctx context.Context) (domain.Topology, error)
	LiveMetrics(ctx context.Context) (domain.LiveMetrics, error)
	Players(ctx context.Context, limit, offset int) (domain.PlayerPage, error)
	Player(ctx context.Context, playerID string) (domain.Player, error)
	Games(ctx context.Context) ([]domain.Game, error)
	CreateGame(ctx context.Context, slug, name string) (*domain.Game, error)
	RenameGame(ctx context.Context, id, name string) (*domain.Game, error)
	DeleteGame(ctx context.Context, id string) error
	GetMultiplayerProfile(ctx context.Context, gameID string) (json.RawMessage, error)
	SetMultiplayerProfile(ctx context.Context, gameID string, profile json.RawMessage) (json.RawMessage, error)
	ListMultiplayerModes(ctx context.Context, gameID string) (json.RawMessage, error)
	GetMultiplayerMode(ctx context.Context, gameID, modeID string) (json.RawMessage, error)
	SetMultiplayerMode(ctx context.Context, gameID, modeID string, mode json.RawMessage) (json.RawMessage, error)
	DeleteMultiplayerMode(ctx context.Context, gameID, modeID string) error
	GameCatalog(ctx context.Context, gameID string) ([]domain.Achievement, error)
	CreateAchievement(ctx context.Context, gameID string, a domain.Achievement) error
	UpdateAchievement(ctx context.Context, gameID, id string, a domain.Achievement) error
	DeleteAchievement(ctx context.Context, gameID, id string) error
	ImportAchievements(ctx context.Context, gameID string, items []domain.Achievement) (domain.ImportResponse, error)
	AchievementProgress(ctx context.Context, gameID, achievementID string) ([]domain.AchievementProgressEntry, error)
	GameEventRules(ctx context.Context, gameID string) ([]domain.EventRule, error)
	CreateEventRule(ctx context.Context, gameID string, r domain.EventRule) (*domain.EventRule, error)
	UpdateEventRule(ctx context.Context, gameID, id string, r domain.EventRule) (*domain.EventRule, error)
	DeleteEventRule(ctx context.Context, gameID, id string) error
	SuppressEventRule(ctx context.Context, gameID, id string) error
	UnsuppressEventRule(ctx context.Context, gameID, id string) error
	PlayerAchievements(ctx context.Context, gameID, playerID string) ([]domain.PlayerAchievement, error)
	GameLeaderboard(ctx context.Context, gameID, board, window string, limit, offset int) ([]domain.LeaderboardEntry, error)
	SetLeaderboardScore(ctx context.Context, gameID, playerID, board string, score int64) (domain.LeaderboardEntry, error)
	PlayerData(ctx context.Context, gameID, playerID string) ([]domain.PlayerDataEntry, error)
	CloudSave(ctx context.Context, gameID, playerID string) (domain.CloudSave, error)
	GamePlayers(ctx context.Context, gameID string) ([]domain.GamePlayer, error)
	PlayerGames(ctx context.Context, playerID string) ([]domain.Game, error)
	GameSessions(ctx context.Context, gameID, state string) (domain.GameSessions, error)
	Rooms(ctx context.Context, state string) (domain.GameSessions, error)
	CloseRoom(ctx context.Context, matchID string) (domain.GameSession, error)
	FanoutHealth(ctx context.Context) (domain.FanoutHealth, error)
	AnalyticsOverview(ctx context.Context) (domain.AnalyticsOverview, error)
	GameAnalytics(ctx context.Context, gameID string, days int) (domain.GameAnalyticsDetail, error)
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
	mux.HandleFunc("GET /api/players/{playerID}", h.player)
	mux.HandleFunc("GET /api/games", h.games)
	mux.HandleFunc("POST /api/games", h.createGame)
	mux.HandleFunc("PUT /api/games/{gameId}", h.renameGame)
	mux.HandleFunc("DELETE /api/games/{gameId}", h.deleteGame)
	mux.HandleFunc("GET /api/games/{gameId}/multiplayer-profile", h.getMultiplayerProfile)
	mux.HandleFunc("PUT /api/games/{gameId}/multiplayer-profile", h.setMultiplayerProfile)
	// Modos nomeados (ADR-0024, spec 017) — passthrough cru da coleção.
	mux.HandleFunc("GET /api/games/{gameId}/multiplayer-profiles", h.listMultiplayerModes)
	mux.HandleFunc("PUT /api/games/{gameId}/multiplayer-profiles/{modeId}", h.setMultiplayerMode)
	mux.HandleFunc("GET /api/games/{gameId}/multiplayer-profiles/{modeId}", h.getMultiplayerMode)
	mux.HandleFunc("DELETE /api/games/{gameId}/multiplayer-profiles/{modeId}", h.deleteMultiplayerMode)
	mux.HandleFunc("GET /api/games/{gameId}/achievements", h.gameCatalog)
	mux.HandleFunc("GET /api/games/{gameId}/achievements/{id}/progress", h.achievementProgress)
	mux.HandleFunc("POST /api/games/{gameId}/achievements/import", h.importAchievements)
	mux.HandleFunc("POST /api/games/{gameId}/achievements", h.createAchievement)
	mux.HandleFunc("PUT /api/games/{gameId}/achievements/{id}", h.updateAchievement)
	mux.HandleFunc("DELETE /api/games/{gameId}/achievements/{id}", h.deleteAchievement)
	mux.HandleFunc("GET /api/games/{gameId}/players/{playerID}/achievements", h.playerAchievements)
	mux.HandleFunc("GET /api/games/{gameId}/event-rules", h.gameEventRules)
	mux.HandleFunc("POST /api/games/{gameId}/event-rules", h.createEventRule)
	mux.HandleFunc("PUT /api/games/{gameId}/event-rules/{id}", h.updateEventRule)
	mux.HandleFunc("DELETE /api/games/{gameId}/event-rules/{id}", h.deleteEventRule)
	mux.HandleFunc("POST /api/games/{gameId}/event-rules/{id}/suppress", h.suppressEventRule)
	mux.HandleFunc("DELETE /api/games/{gameId}/event-rules/{id}/suppress", h.unsuppressEventRule)
	mux.HandleFunc("GET /api/games/{gameId}/leaderboard", h.gameLeaderboard)
	mux.HandleFunc("PUT /api/games/{gameId}/leaderboard/{playerID}/score", h.setLeaderboardScore)
	mux.HandleFunc("GET /api/games/{gameId}/players/{playerID}/data", h.playerData)
	mux.HandleFunc("GET /api/games/{gameId}/players/{playerID}/save", h.cloudSave)
	mux.HandleFunc("GET /api/games/{gameId}/players", h.gamePlayers)
	mux.HandleFunc("GET /api/games/{gameId}/sessions", h.gameSessions)
	mux.HandleFunc("GET /api/players/{playerID}/games", h.playerGames)
	mux.HandleFunc("GET /api/rooms", h.rooms)
	mux.HandleFunc("POST /api/rooms/{matchId}/close", h.closeRoom)
	mux.HandleFunc("GET /api/health/fanout", h.fanoutHealth)
	mux.HandleFunc("GET /api/analytics/overview", h.analyticsOverview)
	mux.HandleFunc("GET /api/analytics/games/{gameId}", h.gameAnalytics)
}

// topology nunca falha por causa do Prometheus: o service degrada (nós unknown).
func (h *Handler) topology(w http.ResponseWriter, r *http.Request) {
	topo, err := h.svc.Topology(r.Context())
	if err != nil {
		httpkit.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}
	httpkit.WriteJSON(w, http.StatusOK, topo)
}

// liveMetrics depende só do Prometheus: fonte fora -> 503.
func (h *Handler) liveMetrics(w http.ResponseWriter, r *http.Request) {
	lm, err := h.svc.LiveMetrics(r.Context())
	if err != nil {
		if errors.Is(err, domain.ErrMetricsUnavailable) {
			httpkit.WriteError(w, http.StatusServiceUnavailable, "metrics_unavailable")
			return
		}
		httpkit.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}
	httpkit.WriteJSON(w, http.StatusOK, lm)
}

// players lista os usuários (User base) via identity. limit/offset repassados;
// identity fora -> 502.
func (h *Handler) players(w http.ResponseWriter, r *http.Request) {
	limit := atoiOr(r.URL.Query().Get("limit"), 0)
	offset := atoiOr(r.URL.Query().Get("offset"), 0)

	page, err := h.svc.Players(r.Context(), limit, offset)
	if err != nil {
		if errors.Is(err, domain.ErrIdentityUnavailable) {
			httpkit.WriteError(w, http.StatusBadGateway, "players_unavailable")
			return
		}
		httpkit.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}
	httpkit.WriteJSON(w, http.StatusOK, page)
}

// player: um usuário pelo id (spec 020) — handle do header do PlayerDetail.
// Inexistente -> 404; identity fora -> 502.
func (h *Handler) player(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.Player(r.Context(), r.PathValue("playerID"))
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrPlayerNotFound):
			httpkit.WriteError(w, http.StatusNotFound, "player_not_found")
		case errors.Is(err, domain.ErrIdentityUnavailable):
			httpkit.WriteError(w, http.StatusBadGateway, "players_unavailable")
		default:
			httpkit.WriteError(w, http.StatusInternalServerError, "internal")
		}
		return
	}
	httpkit.WriteJSON(w, http.StatusOK, p)
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
	httpkit.WriteJSON(w, http.StatusOK, list)
}

// createGame registra um jogo; repassa 400/409 do game-registry.
func (h *Handler) createGame(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Slug string `json:"slug"`
		Name string `json:"name"`
	}
	if err := decode(r, &req); err != nil {
		httpkit.WriteError(w, http.StatusBadRequest, "bad_request")
		return
	}
	g, err := h.svc.CreateGame(r.Context(), req.Slug, req.Name)
	if err != nil {
		writeUpstream(w, err)
		return
	}
	httpkit.WriteJSON(w, http.StatusCreated, g)
}

// renameGame altera o nome do jogo (Configurações do painel, spec 010).
func (h *Handler) renameGame(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := decode(r, &req); err != nil {
		httpkit.WriteError(w, http.StatusBadRequest, "bad_request")
		return
	}
	g, err := h.svc.RenameGame(r.Context(), r.PathValue("gameId"), req.Name)
	if err != nil {
		writeUpstream(w, err)
		return
	}
	httpkit.WriteJSON(w, http.StatusOK, g)
}

// deleteGame remove o jogo (soft delete no registry, ADR-0025, spec 018):
// passthrough com os códigos do upstream intactos (204/404).
func (h *Handler) deleteGame(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteGame(r.Context(), r.PathValue("gameId")); err != nil {
		writeUpstream(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// getMultiplayerProfile lê o perfil de multiplayer do jogo (ADR-0023, spec 012).
// 404 (jogo/perfil ausente) é propagado — a UI trata como "não configurado".
func (h *Handler) getMultiplayerProfile(w http.ResponseWriter, r *http.Request) {
	raw, err := h.svc.GetMultiplayerProfile(r.Context(), r.PathValue("gameId"))
	if err != nil {
		writeUpstream(w, err)
		return
	}
	writeRawJSON(w, http.StatusOK, raw)
}

// setMultiplayerProfile faz upsert do perfil; repassa o corpo cru e os erros de
// validação (400) / não-encontrado (404) do game-registry.
func (h *Handler) setMultiplayerProfile(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		httpkit.WriteError(w, http.StatusBadRequest, "bad_request")
		return
	}
	raw, err := h.svc.SetMultiplayerProfile(r.Context(), r.PathValue("gameId"), body)
	if err != nil {
		writeUpstream(w, err)
		return
	}
	writeRawJSON(w, http.StatusOK, raw)
}

// --- modos nomeados (ADR-0024, spec 017) ---

func (h *Handler) listMultiplayerModes(w http.ResponseWriter, r *http.Request) {
	raw, err := h.svc.ListMultiplayerModes(r.Context(), r.PathValue("gameId"))
	if err != nil {
		writeUpstream(w, err)
		return
	}
	writeRawJSON(w, http.StatusOK, raw)
}

func (h *Handler) getMultiplayerMode(w http.ResponseWriter, r *http.Request) {
	raw, err := h.svc.GetMultiplayerMode(r.Context(), r.PathValue("gameId"), r.PathValue("modeId"))
	if err != nil {
		writeUpstream(w, err)
		return
	}
	writeRawJSON(w, http.StatusOK, raw)
}

func (h *Handler) setMultiplayerMode(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		httpkit.WriteError(w, http.StatusBadRequest, "bad_request")
		return
	}
	raw, err := h.svc.SetMultiplayerMode(r.Context(), r.PathValue("gameId"), r.PathValue("modeId"), body)
	if err != nil {
		writeUpstream(w, err)
		return
	}
	writeRawJSON(w, http.StatusOK, raw)
}

func (h *Handler) deleteMultiplayerMode(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteMultiplayerMode(r.Context(), r.PathValue("gameId"), r.PathValue("modeId")); err != nil {
		writeUpstream(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// gameCatalog lê o catálogo de conquistas do jogo (achievements).
func (h *Handler) gameCatalog(w http.ResponseWriter, r *http.Request) {
	cat, err := h.svc.GameCatalog(r.Context(), r.PathValue("gameId"))
	if err != nil {
		writeUpstream(w, err)
		return
	}
	httpkit.WriteJSON(w, http.StatusOK, cat)
}

// achievementProgress devolve o progresso de todos os jogadores na conquista.
func (h *Handler) achievementProgress(w http.ResponseWriter, r *http.Request) {
	entries, err := h.svc.AchievementProgress(r.Context(), r.PathValue("gameId"), r.PathValue("id"))
	if err != nil {
		writeUpstream(w, err)
		return
	}
	if entries == nil {
		entries = []domain.AchievementProgressEntry{}
	}
	httpkit.WriteJSON(w, http.StatusOK, map[string]any{"progress": entries})
}

// createAchievement cadastra uma conquista no jogo; repassa 400/409.
func (h *Handler) createAchievement(w http.ResponseWriter, r *http.Request) {
	var a domain.Achievement
	if err := decode(r, &a); err != nil {
		httpkit.WriteError(w, http.StatusBadRequest, "bad_request")
		return
	}
	if err := h.svc.CreateAchievement(r.Context(), r.PathValue("gameId"), a); err != nil {
		writeUpstream(w, err)
		return
	}
	httpkit.WriteJSON(w, http.StatusCreated, a)
}

// importAchievements cadastra conquistas em lote a partir de um array JSON
// (spec 011). Sempre 200 — erros por linha estão no corpo da resposta.
func (h *Handler) importAchievements(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Achievements []domain.Achievement `json:"achievements"`
	}
	if err := decode(r, &body); err != nil {
		httpkit.WriteError(w, http.StatusBadRequest, "invalid_input")
		return
	}
	if len(body.Achievements) == 0 {
		httpkit.WriteError(w, http.StatusBadRequest, "empty_batch")
		return
	}
	if len(body.Achievements) > 500 {
		httpkit.WriteError(w, http.StatusBadRequest, "batch_too_large")
		return
	}
	result, err := h.svc.ImportAchievements(r.Context(), r.PathValue("gameId"), body.Achievements)
	if err != nil {
		httpkit.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}
	httpkit.WriteJSON(w, http.StatusOK, result)
}

// updateAchievement edita uma conquista do jogo; repassa 400/404.
func (h *Handler) updateAchievement(w http.ResponseWriter, r *http.Request) {
	var a domain.Achievement
	if err := decode(r, &a); err != nil {
		httpkit.WriteError(w, http.StatusBadRequest, "bad_request")
		return
	}
	if err := h.svc.UpdateAchievement(r.Context(), r.PathValue("gameId"), r.PathValue("id"), a); err != nil {
		writeUpstream(w, err)
		return
	}
	httpkit.WriteJSON(w, http.StatusOK, a)
}

// deleteAchievement remove uma conquista do jogo; repassa 404.
func (h *Handler) deleteAchievement(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteAchievement(r.Context(), r.PathValue("gameId"), r.PathValue("id")); err != nil {
		writeUpstream(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// gameEventRules lê as regras efetivas do jogo (globais do arquétipo +
// overrides do jogo, migration 008).
func (h *Handler) gameEventRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.svc.GameEventRules(r.Context(), r.PathValue("gameId"))
	if err != nil {
		writeUpstream(w, err)
		return
	}
	httpkit.WriteJSON(w, http.StatusOK, rules)
}

// createEventRule cria uma regra escopada ao jogo; repassa 400.
func (h *Handler) createEventRule(w http.ResponseWriter, r *http.Request) {
	var req domain.EventRule
	if err := decode(r, &req); err != nil {
		httpkit.WriteError(w, http.StatusBadRequest, "bad_request")
		return
	}
	rule, err := h.svc.CreateEventRule(r.Context(), r.PathValue("gameId"), req)
	if err != nil {
		writeUpstream(w, err)
		return
	}
	httpkit.WriteJSON(w, http.StatusCreated, rule)
}

// updateEventRule substitui match/eventTypes de uma regra do jogo (spec 016);
// repassa 400 (conteúdo inválido) e 404 (global/inexistente) do upstream.
func (h *Handler) updateEventRule(w http.ResponseWriter, r *http.Request) {
	var req domain.EventRule
	if err := decode(r, &req); err != nil {
		httpkit.WriteError(w, http.StatusBadRequest, "bad_request")
		return
	}
	rule, err := h.svc.UpdateEventRule(r.Context(), r.PathValue("gameId"), r.PathValue("id"), req)
	if err != nil {
		writeUpstream(w, err)
		return
	}
	httpkit.WriteJSON(w, http.StatusOK, rule)
}

// deleteEventRule remove uma regra do jogo (nunca uma global); repassa 404.
func (h *Handler) deleteEventRule(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteEventRule(r.Context(), r.PathValue("gameId"), r.PathValue("id")); err != nil {
		writeUpstream(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// suppressEventRule desliga uma regra global para o jogo (spec 017, V2b);
// repassa 404 (override de jogo ou id inexistente) do upstream.
func (h *Handler) suppressEventRule(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.SuppressEventRule(r.Context(), r.PathValue("gameId"), r.PathValue("id")); err != nil {
		writeUpstream(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// unsuppressEventRule restaura uma regra global suprimida; repassa 404 (sem
// supressão ativa) do upstream.
func (h *Handler) unsuppressEventRule(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.UnsuppressEventRule(r.Context(), r.PathValue("gameId"), r.PathValue("id")); err != nil {
		writeUpstream(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// playerAchievements: achievements do jogador no jogo (achievements).
func (h *Handler) playerAchievements(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.PlayerAchievements(r.Context(), r.PathValue("gameId"), r.PathValue("playerID"))
	if err != nil {
		writeUpstream(w, err)
		return
	}
	httpkit.WriteJSON(w, http.StatusOK, list)
}

// gameLeaderboard lê o top-N do jogo (leaderboard; window/offset repassados
// crus ao upstream, spec 019).
func (h *Handler) gameLeaderboard(w http.ResponseWriter, r *http.Request) {
	limit := atoiOr(r.URL.Query().Get("limit"), 0)
	offset := atoiOr(r.URL.Query().Get("offset"), 0)
	entries, err := h.svc.GameLeaderboard(r.Context(), r.PathValue("gameId"),
		r.URL.Query().Get("board"), r.URL.Query().Get("window"), limit, offset)
	if err != nil {
		writeUpstream(w, err)
		return
	}
	if entries == nil {
		entries = []domain.LeaderboardEntry{}
	}
	httpkit.WriteJSON(w, http.StatusOK, entries)
}

// setLeaderboardScore substitui o score do jogador (admin override, force-set).
func (h *Handler) setLeaderboardScore(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Score *int64 `json:"score"`
	}
	if err := decode(r, &req); err != nil || req.Score == nil {
		httpkit.WriteError(w, http.StatusBadRequest, "invalid_score")
		return
	}
	entry, err := h.svc.SetLeaderboardScore(r.Context(), r.PathValue("gameId"), r.PathValue("playerID"), r.URL.Query().Get("board"), *req.Score)
	if err != nil {
		writeUpstream(w, err)
		return
	}
	httpkit.WriteJSON(w, http.StatusOK, entry)
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
	httpkit.WriteJSON(w, http.StatusOK, players)
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
	httpkit.WriteJSON(w, http.StatusOK, games)
}

// gameSessions: partidas do jogo (session-service), players enriquecidos com
// handle (best-effort no service). state= repassado como filtro.
func (h *Handler) gameSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := h.svc.GameSessions(r.Context(), r.PathValue("gameId"), r.URL.Query().Get("state"))
	if err != nil {
		writeUpstream(w, err)
		return
	}
	if sessions.Active == nil {
		sessions.Active = []domain.GameSession{}
	}
	if sessions.Closed == nil {
		sessions.Closed = []domain.GameSession{}
	}
	httpkit.WriteJSON(w, http.StatusOK, sessions)
}

// rooms é o board global de sessões (todos os jogos), enriquecido. state filtra.
func (h *Handler) rooms(w http.ResponseWriter, r *http.Request) {
	sessions, err := h.svc.Rooms(r.Context(), r.URL.Query().Get("state"))
	if err != nil {
		writeUpstream(w, err)
		return
	}
	if sessions.Active == nil {
		sessions.Active = []domain.GameSession{}
	}
	if sessions.Closed == nil {
		sessions.Closed = []domain.GameSession{}
	}
	httpkit.WriteJSON(w, http.StatusOK, sessions)
}

// closeRoom encerra uma sessão ativa (force-close). 404 do session vira 404.
func (h *Handler) closeRoom(w http.ResponseWriter, r *http.Request) {
	sess, err := h.svc.CloseRoom(r.Context(), r.PathValue("matchId"))
	if err != nil {
		writeUpstream(w, err)
		return
	}
	httpkit.WriteJSON(w, http.StatusOK, sess)
}

// analyticsOverview expõe o panorama cross-game (ADR-0021). Analytics fora -> 502.
func (h *Handler) analyticsOverview(w http.ResponseWriter, r *http.Request) {
	ov, err := h.svc.AnalyticsOverview(r.Context())
	if err != nil {
		if errors.Is(err, domain.ErrAnalyticsUnavailable) {
			httpkit.WriteError(w, http.StatusBadGateway, "analytics_unavailable")
			return
		}
		httpkit.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}
	if ov.Games == nil {
		ov.Games = []domain.AnalyticsGameStat{}
	}
	if ov.Daily == nil {
		ov.Daily = []domain.AnalyticsDaily{}
	}
	httpkit.WriteJSON(w, http.StatusOK, ov)
}

// gameAnalytics expõe o detalhe de analytics de um jogo (resumo + série diária +
// participações por papel, host_world_dropin — spec 014). Analytics fora -> 502.
func (h *Handler) gameAnalytics(w http.ResponseWriter, r *http.Request) {
	days := 30
	if q := r.URL.Query().Get("days"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 && n <= 365 {
			days = n
		}
	}
	d, err := h.svc.GameAnalytics(r.Context(), r.PathValue("gameId"), days)
	if err != nil {
		if errors.Is(err, domain.ErrAnalyticsUnavailable) {
			httpkit.WriteError(w, http.StatusBadGateway, "analytics_unavailable")
			return
		}
		httpkit.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}
	if d.Daily == nil {
		d.Daily = []domain.AnalyticsDaily{}
	}
	if d.RoleParticipations == nil {
		d.RoleParticipations = []domain.ParticipationStat{}
	}
	httpkit.WriteJSON(w, http.StatusOK, d)
}

// fanoutHealth expõe os sinais do fan-out durável (ADR-0020). Prometheus fora -> 503.
func (h *Handler) fanoutHealth(w http.ResponseWriter, r *http.Request) {
	fh, err := h.svc.FanoutHealth(r.Context())
	if err != nil {
		if errors.Is(err, domain.ErrMetricsUnavailable) {
			httpkit.WriteError(w, http.StatusServiceUnavailable, "metrics_unavailable")
			return
		}
		httpkit.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}
	httpkit.WriteJSON(w, http.StatusOK, fh)
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
	httpkit.WriteJSON(w, http.StatusOK, entries)
}

// cloudSave: save do jogador no jogo (cloud-save). 404 (sem save) é repassado
// via UpstreamError pelo writeUpstream.
func (h *Handler) cloudSave(w http.ResponseWriter, r *http.Request) {
	save, err := h.svc.CloudSave(r.Context(), r.PathValue("gameId"), r.PathValue("playerID"))
	if err != nil {
		writeUpstream(w, err)
		return
	}
	httpkit.WriteJSON(w, http.StatusOK, save)
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
		httpkit.WriteError(w, ue.Status, code)
	case errors.Is(err, domain.ErrGameRegistryUnavailable):
		httpkit.WriteError(w, http.StatusBadGateway, "game_registry_unavailable")
	case errors.Is(err, domain.ErrAchievementsUnavailable):
		httpkit.WriteError(w, http.StatusBadGateway, "achievements_unavailable")
	case errors.Is(err, domain.ErrLeaderboardUnavailable):
		httpkit.WriteError(w, http.StatusBadGateway, "leaderboard_unavailable")
	case errors.Is(err, domain.ErrPlayerDataUnavailable):
		httpkit.WriteError(w, http.StatusBadGateway, "player_data_unavailable")
	case errors.Is(err, domain.ErrCloudSaveUnavailable):
		httpkit.WriteError(w, http.StatusBadGateway, "cloud_save_unavailable")
	case errors.Is(err, domain.ErrSessionsUnavailable):
		httpkit.WriteError(w, http.StatusBadGateway, "sessions_unavailable")
	default:
		httpkit.WriteError(w, http.StatusInternalServerError, "internal")
	}
}

// writeRawJSON repassa um corpo JSON já serializado (passthrough cru, sem
// re-decodificar) — usado no perfil de multiplayer (spec 012).
func writeRawJSON(w http.ResponseWriter, status int, raw json.RawMessage) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(raw)
}
