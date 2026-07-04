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

	"github.com/vitormartins1/go-game-backend/services/achievements-service/internal/domain"
)

// Handler expõe os endpoints HTTP do achievements-service.
type Handler struct {
	svc      domain.AchievementService
	verifier domain.TokenVerifier
}

// NewHandler cria o handler a partir das Ports do domínio.
func NewHandler(svc domain.AchievementService, verifier domain.TokenVerifier) *Handler {
	return &Handler{svc: svc, verifier: verifier}
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
}

// auth valida o Bearer JWT (via TokenVerifier) e injeta o playerID no contexto.
func (h *Handler) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
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
		Target      int    `json:"target"`
	}
	if err := decodeStrict(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	a, err := h.svc.CreateAchievement(r.Context(), r.PathValue("gameId"), domain.Achievement{
		ID: req.ID, Name: req.Name, Description: req.Description, EventType: req.EventType, Target: req.Target,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, achievementDTO{ID: a.ID, Name: a.Name, Description: a.Description, EventType: a.EventType, Target: a.Target})
}

// adminUpdateAchievement edita uma conquista do catálogo do jogo (admin).
func (h *Handler) adminUpdateAchievement(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		EventType   string `json:"eventType"`
		Target      int    `json:"target"`
	}
	if err := decodeStrict(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	a, err := h.svc.UpdateAchievement(r.Context(), r.PathValue("gameId"), r.PathValue("id"), domain.Achievement{
		Name: req.Name, Description: req.Description, EventType: req.EventType, Target: req.Target,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, achievementDTO{ID: a.ID, Name: a.Name, Description: a.Description, EventType: a.EventType, Target: a.Target})
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

func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return h[len(prefix):]
	}
	return ""
}

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
	EventType   string `json:"eventType"` // exposto para o painel editar (006-catalog-crud)
	Target      int    `json:"target"`
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

func toAchievementDTOs(in []domain.Achievement) []achievementDTO {
	out := make([]achievementDTO, 0, len(in))
	for _, a := range in {
		out = append(out, achievementDTO{ID: a.ID, Name: a.Name, Description: a.Description, EventType: a.EventType, Target: a.Target})
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
	default:
		writeError(w, http.StatusInternalServerError, "internal")
	}
}
