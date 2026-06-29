// Package transport traduz HTTP para o domínio e vice-versa (handlers do
// leaderboard-service). Sem lógica de negócio. Ver ADR-0002.
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/c130n/game-backend/services/leaderboard-service/internal/domain"
)

// Handler expõe os endpoints HTTP do leaderboard-service.
type Handler struct {
	svc      domain.LeaderboardService
	verifier domain.TokenVerifier
}

// NewHandler cria o handler a partir das Ports do domínio.
func NewHandler(svc domain.LeaderboardService, verifier domain.TokenVerifier) *Handler {
	return &Handler{svc: svc, verifier: verifier}
}

type ctxKey int

const playerIDKey ctxKey = iota

// Register monta as rotas, todas atrás do auth.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.Handle("POST /scores", h.auth(http.HandlerFunc(h.submit)))
	mux.Handle("GET /leaderboard", h.auth(http.HandlerFunc(h.top)))
	mux.Handle("GET /leaderboard/me", h.auth(http.HandlerFunc(h.me)))
}

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

func (h *Handler) submit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Score *int64 `json:"score"` // ponteiro: distingue ausente de zero
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Score == nil {
		writeError(w, http.StatusBadRequest, "invalid_score")
		return
	}
	entry, err := h.svc.Submit(r.Context(), playerID(r.Context()), *req.Score)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toDTO(entry))
}

func (h *Handler) top(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit")) // 0/erro -> service aplica o default
	entries, err := h.svc.Top(r.Context(), limit)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": toDTOs(entries)})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	k := -1 // ausente -> service aplica o default; "0" é válido (só o jogador)
	if v := r.URL.Query().Get("around"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			k = n
		}
	}
	entries, err := h.svc.Around(r.Context(), playerID(r.Context()), k)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": toDTOs(entries)})
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

type entryDTO struct {
	Rank     int64  `json:"rank"`
	PlayerID string `json:"playerId"`
	Score    int64  `json:"score"`
}

func toDTO(e domain.Entry) entryDTO {
	return entryDTO{Rank: e.Rank, PlayerID: e.PlayerID, Score: e.Score}
}

func toDTOs(in []domain.Entry) []entryDTO {
	out := make([]entryDTO, 0, len(in))
	for _, e := range in {
		out = append(out, toDTO(e))
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

func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotRanked):
		writeError(w, http.StatusNotFound, "not_ranked")
	case errors.Is(err, domain.ErrInvalidScore):
		writeError(w, http.StatusBadRequest, "invalid_score")
	default:
		writeError(w, http.StatusInternalServerError, "internal")
	}
}
