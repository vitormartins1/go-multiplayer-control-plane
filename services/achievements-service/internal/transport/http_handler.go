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

// Register monta as rotas no mux, todas atrás do middleware de auth.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.Handle("POST /events", h.auth(http.HandlerFunc(h.recordEvent)))
	mux.Handle("GET /achievements", h.auth(http.HandlerFunc(h.listCatalog)))
	mux.Handle("GET /me/achievements", h.auth(http.HandlerFunc(h.listForPlayer)))
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
}

type playerAchievementDTO struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Unlocked    bool       `json:"unlocked"`
	UnlockedAt  *time.Time `json:"unlocked_at,omitempty"`
}

func toAchievementDTOs(in []domain.Achievement) []achievementDTO {
	out := make([]achievementDTO, 0, len(in))
	for _, a := range in {
		out = append(out, achievementDTO{ID: a.ID, Name: a.Name, Description: a.Description})
	}
	return out
}

func toPlayerAchievementDTOs(in []domain.PlayerAchievement) []playerAchievementDTO {
	out := make([]playerAchievementDTO, 0, len(in))
	for _, p := range in {
		dto := playerAchievementDTO{ID: p.ID, Name: p.Name, Description: p.Description, Unlocked: p.Unlocked}
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
	case errors.Is(err, domain.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, "unauthorized")
	case errors.Is(err, domain.ErrInvalidEvent):
		writeError(w, http.StatusBadRequest, "invalid_event")
	default:
		writeError(w, http.StatusInternalServerError, "internal")
	}
}
