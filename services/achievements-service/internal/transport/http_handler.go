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

	"github.com/c130n/game-backend/services/achievements-service/internal/domain"
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

// Register monta as rotas no mux. As de jogador ficam atrás do middleware de
// auth; as de /admin são control plane interno (sem auth no MVP — ver
// specs/003-admin-read): a barreira é a rede, não JWT.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.Handle("POST /events", h.auth(http.HandlerFunc(h.recordEvent)))
	mux.Handle("GET /achievements", h.auth(http.HandlerFunc(h.listCatalog)))
	mux.Handle("GET /me/achievements", h.auth(http.HandlerFunc(h.listForPlayer)))

	// Admin (interno, sem auth): o playerID vem do path, não do JWT.
	mux.HandleFunc("GET /admin/players/{playerID}/achievements", h.adminListForPlayer)
	mux.HandleFunc("GET /admin/achievements", h.adminListCatalog)
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
	var req struct {
		Type string `json:"type"`
	}
	// sem DisallowUnknownFields de propósito: um id de jogador no corpo é
	// ignorado (o playerID vem do JWT), e o evento é extensível.
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	unlocked, err := h.svc.RecordEvent(r.Context(), playerID(r.Context()), domain.Event{Type: req.Type})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"unlocked": toAchievementDTOs(unlocked)})
}

func (h *Handler) listCatalog(w http.ResponseWriter, r *http.Request) {
	cat, err := h.svc.ListCatalog(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"achievements": toAchievementDTOs(cat)})
}

func (h *Handler) listForPlayer(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.ListForPlayer(r.Context(), playerID(r.Context()))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"achievements": toPlayerAchievementDTOs(list)})
}

// adminListForPlayer devolve o catálogo anotado com o progresso do jogador
// identificado pelo path (endpoint admin, sem JWT). Reusa a mesma lógica do
// /me/achievements, só mudando de onde vem o playerID.
func (h *Handler) adminListForPlayer(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.ListForPlayer(r.Context(), r.PathValue("playerID"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"achievements": toPlayerAchievementDTOs(list)})
}

// adminListCatalog devolve o catálogo de conquistas (endpoint admin, sem JWT).
func (h *Handler) adminListCatalog(w http.ResponseWriter, r *http.Request) {
	cat, err := h.svc.ListCatalog(r.Context())
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

type achievementDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Target      int    `json:"target"`
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
		out = append(out, achievementDTO{ID: a.ID, Name: a.Name, Description: a.Description, Target: a.Target})
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
	default:
		writeError(w, http.StatusInternalServerError, "internal")
	}
}
