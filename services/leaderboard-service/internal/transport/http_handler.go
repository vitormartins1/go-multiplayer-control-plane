// Package transport traduz HTTP para o domínio e vice-versa (handlers do
// leaderboard-service). Sem lógica de negócio. Ver ADR-0002.
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/leaderboard-service/internal/domain"
	"github.com/vitormartins1/go-multiplayer-control-plane/shared/auth"
	"github.com/vitormartins1/go-multiplayer-control-plane/shared/httpkit"
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

// Register monta as rotas. Player-facing atrás do JWT (jogo no header X-Game-Id);
// admin (internal key na borda, ADR-0018) com jogo/jogador no path.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.Handle("POST /scores", h.auth(http.HandlerFunc(h.submit)))
	mux.Handle("GET /leaderboard", h.auth(http.HandlerFunc(h.top)))
	mux.Handle("GET /leaderboard/me", h.auth(http.HandlerFunc(h.me)))

	mux.HandleFunc("POST /admin/games/{gameId}/players/{playerID}/scores", h.adminSubmit)
	mux.HandleFunc("PUT /admin/games/{gameId}/players/{playerID}/scores", h.adminForceScore)
	mux.HandleFunc("GET /admin/games/{gameId}/leaderboard", h.adminTop)
}

func (h *Handler) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := auth.BearerToken(r.Header.Get("Authorization"))
		if token == "" {
			httpkit.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		playerID, err := h.verifier.Verify(r.Context(), token)
		if err != nil {
			httpkit.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		ctx := context.WithValue(r.Context(), playerIDKey, playerID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *Handler) submit(w http.ResponseWriter, r *http.Request) {
	gameID := gameFromHeader(r)
	if gameID == "" {
		httpkit.WriteError(w, http.StatusBadRequest, "missing_game")
		return
	}
	var req struct {
		Score *int64 `json:"score"` // ponteiro: distingue ausente de zero
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Score == nil {
		httpkit.WriteError(w, http.StatusBadRequest, "invalid_score")
		return
	}
	entry, err := h.svc.Submit(r.Context(), gameID, boardFromQuery(r), playerID(r.Context()), *req.Score)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	httpkit.WriteJSON(w, http.StatusOK, toDTO(entry))
}

func (h *Handler) top(w http.ResponseWriter, r *http.Request) {
	gameID := gameFromHeader(r)
	if gameID == "" {
		httpkit.WriteError(w, http.StatusBadRequest, "missing_game")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit")) // 0/erro -> service aplica o default
	entries, err := h.svc.Top(r.Context(), gameID, boardFromQuery(r), windowFromQuery(r), limit, offsetFromQuery(r))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	httpkit.WriteJSON(w, http.StatusOK, map[string]any{"entries": toDTOs(entries)})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	gameID := gameFromHeader(r)
	if gameID == "" {
		httpkit.WriteError(w, http.StatusBadRequest, "missing_game")
		return
	}
	k := -1 // ausente -> service aplica o default; "0" é válido (só o jogador)
	if v := r.URL.Query().Get("around"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			k = n
		}
	}
	entries, err := h.svc.Around(r.Context(), gameID, boardFromQuery(r), windowFromQuery(r), playerID(r.Context()), k)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	httpkit.WriteJSON(w, http.StatusOK, map[string]any{"entries": toDTOs(entries)})
}

// adminSubmit aplica o highscore de um jogador no ranking do jogo
// (server-authoritative: jogo/jogador no path, sem JWT). É o que o fan-out usa.
func (h *Handler) adminSubmit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Score *int64 `json:"score"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Score == nil {
		httpkit.WriteError(w, http.StatusBadRequest, "invalid_score")
		return
	}
	entry, err := h.svc.Submit(r.Context(), r.PathValue("gameId"), boardFromQuery(r), r.PathValue("playerID"), *req.Score)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	httpkit.WriteJSON(w, http.StatusOK, toDTO(entry))
}

// adminForceScore substitui o score do jogador independente do valor atual (admin override).
func (h *Handler) adminForceScore(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Score *int64 `json:"score"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Score == nil {
		httpkit.WriteError(w, http.StatusBadRequest, "invalid_score")
		return
	}
	entry, err := h.svc.ForceSubmit(r.Context(), r.PathValue("gameId"), boardFromQuery(r), r.PathValue("playerID"), *req.Score)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	httpkit.WriteJSON(w, http.StatusOK, toDTO(entry))
}

// adminTop devolve o top-N do jogo (consumo do painel).
func (h *Handler) adminTop(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	entries, err := h.svc.Top(r.Context(), r.PathValue("gameId"), boardFromQuery(r), windowFromQuery(r), limit, offsetFromQuery(r))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	httpkit.WriteJSON(w, http.StatusOK, map[string]any{"entries": toDTOs(entries)})
}

// --- helpers ---

func playerID(ctx context.Context) string {
	id, _ := ctx.Value(playerIDKey).(string)
	return id
}

// gameFromHeader lê o jogo do header X-Game-Id (contexto do cliente, ADR-0017).
func gameFromHeader(r *http.Request) string {
	return r.Header.Get("X-Game-Id")
}

// boardFromQuery lê o board do query param `?board`; ausente -> "score" (ADR-0023).
func boardFromQuery(r *http.Request) string {
	if b := r.URL.Query().Get("board"); b != "" {
		return b
	}
	return domain.BoardScore
}

// windowFromQuery lê a janela do query param `?window`; ausente -> all-time
// (spec 007 — compat total com as leituras pré-janelas).
func windowFromQuery(r *http.Request) string {
	if w := r.URL.Query().Get("window"); w != "" {
		return w
	}
	return domain.WindowAllTime
}

// offsetFromQuery lê a página do query param `?offset`; ausente/erro -> 0
// (service clampa negativo).
func offsetFromQuery(r *http.Request) int {
	n, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	return n
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

func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotRanked):
		httpkit.WriteError(w, http.StatusNotFound, "not_ranked")
	case errors.Is(err, domain.ErrInvalidScore):
		httpkit.WriteError(w, http.StatusBadRequest, "invalid_score")
	case errors.Is(err, domain.ErrInvalidBoard):
		httpkit.WriteError(w, http.StatusBadRequest, "invalid_board")
	case errors.Is(err, domain.ErrInvalidWindow):
		httpkit.WriteError(w, http.StatusBadRequest, "invalid_window")
	default:
		httpkit.WriteError(w, http.StatusInternalServerError, "internal")
	}
}
