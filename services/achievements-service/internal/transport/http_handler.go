// Package transport traduz HTTP para o domínio e vice-versa (handlers do
// achievements-service). Sem lógica de negócio. Ver ADR-0002.
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/achievements-service/internal/domain"
	"github.com/vitormartins1/go-multiplayer-control-plane/shared/auth"
)

// Handler expõe os endpoints HTTP do achievements-service.
type Handler struct {
	svc      domain.AchievementService
	rules    domain.EventRuleService
	verifier domain.TokenVerifier
}

// NewHandler cria o handler a partir das Ports do domínio. svc e rules
// tipicamente apontam pro mesmo *service.Service (que implementa as duas
// Ports), passados separados aqui pra manter o Handler dependendo só das
// interfaces que de fato usa (ADR-0002).
func NewHandler(svc domain.AchievementService, rules domain.EventRuleService, verifier domain.TokenVerifier) *Handler {
	return &Handler{svc: svc, rules: rules, verifier: verifier}
}

type ctxKey int

const playerIDKey ctxKey = iota

// Register monta as rotas no mux. Multi-tenant por jogo (ADR-0017): nas rotas
// de jogador o game_id vem do header X-Game-Id (contexto ambiente); nas de
// /admin vem do path {gameId}. As de jogador ficam atrás do auth; as de /admin
// são control plane interno (sem auth no MVP).
func (h *Handler) Register(mux *http.ServeMux) {
	mux.Handle("POST /events", h.auth(http.HandlerFunc(h.recordEvent)))
	mux.Handle("GET /achievements", h.auth(http.HandlerFunc(h.listCatalog)))
	mux.Handle("GET /me/achievements", h.auth(http.HandlerFunc(h.listForPlayer)))

	// Admin (interno; internal key na borda, ADR-0018) por jogo: gameId/playerID
	// vêm do path.
	mux.HandleFunc("POST /admin/games/{gameId}/achievements", h.adminCreateAchievement)
	mux.HandleFunc("PUT /admin/games/{gameId}/achievements/{id}", h.adminUpdateAchievement)
	mux.HandleFunc("DELETE /admin/games/{gameId}/achievements/{id}", h.adminDeleteAchievement)
	mux.HandleFunc("GET /admin/games/{gameId}/achievements", h.adminListCatalog)
	mux.HandleFunc("GET /admin/games/{gameId}/players/{playerID}/achievements", h.adminListForPlayer)
	// Server-authoritative: o servidor reporta evento em nome do jogador (sem JWT).
	mux.HandleFunc("POST /admin/games/{gameId}/players/{playerID}/events", h.adminRecordEvent)
	// Progresso de todos os jogadores em uma conquista (painel admin).
	mux.HandleFunc("GET /admin/games/{gameId}/achievements/{id}/progress", h.adminAchievementProgress)

	// Regras de mapeamento evento->achievement-event-type (migration 008):
	// globais do arquétipo (read-only por aqui) + overrides do jogo (CRUD).
	mux.HandleFunc("GET /admin/games/{gameId}/event-rules", h.adminListEventRules)
	mux.HandleFunc("POST /admin/games/{gameId}/event-rules", h.adminCreateEventRule)
	mux.HandleFunc("PUT /admin/games/{gameId}/event-rules/{id}", h.adminUpdateEventRule)
	mux.HandleFunc("DELETE /admin/games/{gameId}/event-rules/{id}", h.adminDeleteEventRule)
	// Supressão de regra global por jogo (spec 017, V2b do ADR-0028).
	mux.HandleFunc("POST /admin/games/{gameId}/event-rules/{id}/suppress", h.adminSuppressEventRule)
	mux.HandleFunc("DELETE /admin/games/{gameId}/event-rules/{id}/suppress", h.adminUnsuppressEventRule)
}

// auth valida o Bearer JWT (via TokenVerifier) e injeta o playerID no contexto.
func (h *Handler) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := auth.BearerToken(r.Header.Get("Authorization"))
		if token == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		playerID, err := h.verifier.Verify(r.Context(), token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		ctx := context.WithValue(r.Context(), playerIDKey, playerID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *Handler) recordEvent(w http.ResponseWriter, r *http.Request) {
	gameID := gameHeader(r)
	if gameID == "" {
		writeError(w, http.StatusBadRequest, "missing_game")
		return
	}
	var req struct {
		Type string `json:"type"`
	}
	// sem DisallowUnknownFields de propósito: um id de jogador no corpo é
	// ignorado (o playerID vem do JWT), e o evento é extensível.
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	unlocked, err := h.svc.RecordEvent(r.Context(), gameID, playerID(r.Context()), domain.Event{Type: req.Type})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"unlocked": toAchievementDTOs(unlocked)})
}

func (h *Handler) listCatalog(w http.ResponseWriter, r *http.Request) {
	gameID := gameHeader(r)
	if gameID == "" {
		writeError(w, http.StatusBadRequest, "missing_game")
		return
	}
	cat, err := h.svc.ListCatalog(r.Context(), gameID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"achievements": toAchievementDTOs(cat)})
}

func (h *Handler) listForPlayer(w http.ResponseWriter, r *http.Request) {
	gameID := gameHeader(r)
	if gameID == "" {
		writeError(w, http.StatusBadRequest, "missing_game")
		return
	}
	list, err := h.svc.ListForPlayer(r.Context(), gameID, playerID(r.Context()))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"achievements": toPlayerAchievementDTOs(list)})
}

// adminCreateAchievement adiciona uma conquista ao catálogo do jogo do path.
func (h *Handler) adminCreateAchievement(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		EventType   string `json:"eventType"`
		StatKey     string `json:"statKey"`
		StatMode    string `json:"statMode"`
		Target      int    `json:"target"`
	}
	if err := decodeStrict(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	a, err := h.svc.CreateAchievement(r.Context(), r.PathValue("gameId"), domain.Achievement{
		ID: req.ID, Name: req.Name, Description: req.Description,
		EventType: req.EventType, StatKey: req.StatKey, StatMode: req.StatMode, Target: req.Target,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toAchievementDTO(*a))
}

// adminUpdateAchievement edita uma conquista do catálogo do jogo (admin).
func (h *Handler) adminUpdateAchievement(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		EventType   string `json:"eventType"`
		StatKey     string `json:"statKey"`
		StatMode    string `json:"statMode"`
		Target      int    `json:"target"`
	}
	if err := decodeStrict(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	a, err := h.svc.UpdateAchievement(r.Context(), r.PathValue("gameId"), r.PathValue("id"), domain.Achievement{
		Name: req.Name, Description: req.Description,
		EventType: req.EventType, StatKey: req.StatKey, StatMode: req.StatMode, Target: req.Target,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toAchievementDTO(*a))
}

// adminDeleteAchievement remove uma conquista do catálogo do jogo (admin).
func (h *Handler) adminDeleteAchievement(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteAchievement(r.Context(), r.PathValue("gameId"), r.PathValue("id")); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// adminRecordEvent registra um evento de gameplay em nome do jogador (server-
// authoritative): gameId/playerID vêm do path, não do JWT. É o que o fan-out do
// session usa. Reusa a mesma avaliação do POST /events de jogador.
func (h *Handler) adminRecordEvent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	unlocked, err := h.svc.RecordEvent(r.Context(), r.PathValue("gameId"), r.PathValue("playerID"), domain.Event{Type: req.Type})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"unlocked": toAchievementDTOs(unlocked)})
}

// adminListForPlayer devolve o catálogo do jogo {gameId} anotado com o progresso
// do jogador {playerID} (endpoint admin, sem JWT).
func (h *Handler) adminListForPlayer(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.ListForPlayer(r.Context(), r.PathValue("gameId"), r.PathValue("playerID"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"achievements": toPlayerAchievementDTOs(list)})
}

// adminAchievementProgress devolve o progresso de todos os jogadores na conquista.
func (h *Handler) adminAchievementProgress(w http.ResponseWriter, r *http.Request) {
	entries, err := h.svc.ProgressForAchievement(r.Context(), r.PathValue("gameId"), r.PathValue("id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	dtos := make([]achievementProgressDTO, 0, len(entries))
	for _, e := range entries {
		dto := achievementProgressDTO{PlayerID: e.PlayerID, Progress: e.Progress, Unlocked: e.Unlocked}
		if e.Unlocked {
			at := e.UnlockedAt
			dto.UnlockedAt = &at
		}
		dtos = append(dtos, dto)
	}
	writeJSON(w, http.StatusOK, map[string]any{"progress": dtos})
}

// adminListEventRules devolve as regras EFETIVAS do jogo {gameId}: globais de
// cada arquétipo (scope="global") + overrides específicos do jogo
// (scope="game").
func (h *Handler) adminListEventRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.rules.ListEffectiveRules(r.Context(), r.PathValue("gameId"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": toEventRuleDTOs(rules)})
}

// adminCreateEventRule cria uma regra escopada ao jogo {gameId} (nunca uma
// global — gameId/scope vêm do path/servidor, não do payload).
func (h *Handler) adminCreateEventRule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Archetype  string                      `json:"archetype"`
		Match      map[string]domain.MatchCond `json:"match"` // string ou {op,value} (spec 014)
		EventTypes []string                    `json:"eventTypes"`
	}
	if err := decodeStrict(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	rule, err := h.rules.CreateEventRule(r.Context(), r.PathValue("gameId"), domain.EventRule{
		Archetype: req.Archetype, Match: req.Match, EventTypes: req.EventTypes,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toEventRuleDTO(*rule))
}

// adminUpdateEventRule substitui match/eventTypes de uma regra do jogo
// {gameId} (spec 016). Archetype não é aceito no payload (não editável) —
// decodeStrict rejeita campo desconhecido com 400.
func (h *Handler) adminUpdateEventRule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Match      map[string]domain.MatchCond `json:"match"` // string ou {op,value} (spec 014)
		EventTypes []string                    `json:"eventTypes"`
	}
	if err := decodeStrict(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	rule, err := h.rules.UpdateEventRule(r.Context(), r.PathValue("gameId"), r.PathValue("id"), domain.EventRule{
		Match: req.Match, EventTypes: req.EventTypes,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toEventRuleDTO(*rule))
}

// adminDeleteEventRule remove uma regra do jogo {gameId} (globais são
// protegidas — DeleteEventRule/ErrEventRuleNotFound trata como 404).
func (h *Handler) adminDeleteEventRule(w http.ResponseWriter, r *http.Request) {
	if err := h.rules.DeleteEventRule(r.Context(), r.PathValue("gameId"), r.PathValue("id")); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// adminSuppressEventRule desliga uma regra GLOBAL para o jogo {gameId} (spec
// 017, V2b) — a linha da regra não é tocada; só deixa de contribuir event
// types nas avaliações deste jogo. Idempotente (204 mesmo se já suprimida).
// ErrEventRuleNotFound (regra inexistente ou não-global) -> 404.
func (h *Handler) adminSuppressEventRule(w http.ResponseWriter, r *http.Request) {
	if err := h.rules.SuppressEventRule(r.Context(), r.PathValue("gameId"), r.PathValue("id")); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// adminUnsuppressEventRule restaura uma regra global suprimida. Sem
// supressão ativa para o par (gameId, id) -> ErrEventRuleNotFound -> 404.
func (h *Handler) adminUnsuppressEventRule(w http.ResponseWriter, r *http.Request) {
	if err := h.rules.UnsuppressEventRule(r.Context(), r.PathValue("gameId"), r.PathValue("id")); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// adminListCatalog devolve o catálogo de conquistas do jogo {gameId} (sem JWT).
func (h *Handler) adminListCatalog(w http.ResponseWriter, r *http.Request) {
	cat, err := h.svc.ListCatalog(r.Context(), r.PathValue("gameId"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"achievements": toAchievementDTOs(cat)})
}

// --- helpers ---

func playerID(ctx context.Context) string {
	id, _ := ctx.Value(playerIDKey).(string)
	return id
}

// gameHeader lê o jogo do header X-Game-Id (contexto ambiente do data plane).
func gameHeader(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get("X-Game-Id"))
}

// decodeStrict decodifica o corpo rejeitando campos desconhecidos.
func decodeStrict(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

type achievementDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	EventType   string `json:"eventType"`          // critério por evento (006-catalog-crud)
	StatKey     string `json:"statKey,omitempty"`  // critério por stat (ADR-0023 A1)
	StatMode    string `json:"statMode,omitempty"` // cumulative | per_match
	Target      int    `json:"target"`
}

type eventRuleDTO struct {
	ID         string                      `json:"id"`
	Scope      string                      `json:"scope"` // "global" | "game"
	Archetype  string                      `json:"archetype"`
	Match      map[string]domain.MatchCond `json:"match"` // eq serializa como string (compat ADR-0027)
	EventTypes []string                    `json:"eventTypes"`
	// Suppressed (spec 017, V2b): sempre presente; só relevante pra scope
	// "global" — regras de jogo nunca são suprimíveis (o conceito não se
	// aplica a overrides, que são removíveis via DELETE).
	Suppressed bool `json:"suppressed"`
}

func toEventRuleDTO(r domain.EventRule) eventRuleDTO {
	scope := "game"
	if r.GameID == nil {
		scope = "global"
	}
	match := r.Match
	if match == nil {
		match = map[string]domain.MatchCond{}
	}
	return eventRuleDTO{ID: r.ID, Scope: scope, Archetype: r.Archetype, Match: match, EventTypes: r.EventTypes, Suppressed: r.Suppressed}
}

func toEventRuleDTOs(in []domain.EventRule) []eventRuleDTO {
	out := make([]eventRuleDTO, 0, len(in))
	for _, r := range in {
		out = append(out, toEventRuleDTO(r))
	}
	return out
}

type achievementProgressDTO struct {
	PlayerID   string     `json:"playerId"`
	Progress   int        `json:"progress"`
	Unlocked   bool       `json:"unlocked"`
	UnlockedAt *time.Time `json:"unlockedAt,omitempty"`
}

type playerAchievementDTO struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Progress    int        `json:"progress"`
	Target      int        `json:"target"`
	Unlocked    bool       `json:"unlocked"`
	UnlockedAt  *time.Time `json:"unlocked_at,omitempty"`
}

func toAchievementDTO(a domain.Achievement) achievementDTO {
	return achievementDTO{
		ID: a.ID, Name: a.Name, Description: a.Description,
		EventType: a.EventType, StatKey: a.StatKey, StatMode: a.StatMode, Target: a.Target,
	}
}

func toAchievementDTOs(in []domain.Achievement) []achievementDTO {
	out := make([]achievementDTO, 0, len(in))
	for _, a := range in {
		out = append(out, toAchievementDTO(a))
	}
	return out
}

func toPlayerAchievementDTOs(in []domain.PlayerAchievement) []playerAchievementDTO {
	out := make([]playerAchievementDTO, 0, len(in))
	for _, p := range in {
		dto := playerAchievementDTO{
			ID: p.ID, Name: p.Name, Description: p.Description,
			Progress: p.Progress, Target: p.Target, Unlocked: p.Unlocked,
		}
		if p.Unlocked {
			at := p.UnlockedAt
			dto.UnlockedAt = &at
		}
		out = append(out, dto)
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

// writeDomainError mapeia erros de domínio para HTTP status + código estável.
func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidEvent):
		writeError(w, http.StatusBadRequest, "invalid_event")
	case errors.Is(err, domain.ErrInvalidAchievement):
		writeError(w, http.StatusBadRequest, "invalid_achievement")
	case errors.Is(err, domain.ErrAchievementExists):
		writeError(w, http.StatusConflict, "achievement_exists")
	case errors.Is(err, domain.ErrAchievementNotFound):
		writeError(w, http.StatusNotFound, "achievement_not_found")
	case errors.Is(err, domain.ErrInvalidEventRule):
		writeError(w, http.StatusBadRequest, "invalid_event_rule")
	case errors.Is(err, domain.ErrEventRuleNotFound):
		writeError(w, http.StatusNotFound, "event_rule_not_found")
	default:
		writeError(w, http.StatusInternalServerError, "internal")
	}
}
