// Package transport traduz HTTP para o domínio e vice-versa (handlers do
// player-data-service). Sem lógica de negócio. Ver ADR-0002.
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/player-data-service/internal/domain"
	"github.com/vitormartins1/go-multiplayer-control-plane/shared/auth"
)

// Handler expõe os endpoints HTTP do player-data-service.
type Handler struct {
	svc      domain.PlayerDataService
	verifier domain.TokenVerifier
}

// NewHandler cria o handler a partir das Ports do domínio.
func NewHandler(svc domain.PlayerDataService, verifier domain.TokenVerifier) *Handler {
	return &Handler{svc: svc, verifier: verifier}
}

type ctxKey int

const playerIDKey ctxKey = iota

// Register monta as rotas (path params do Go 1.22). As de jogador ficam atrás
// do auth (JWT) e pegam o jogo do header X-Game-Id; a admin (jogo/jogador no
// path) fica fora do JWT — protegida pela internal key no main (ADR-0018).
func (h *Handler) Register(mux *http.ServeMux) {
	mux.Handle("PUT /me/data/{namespace}/{key}", h.auth(http.HandlerFunc(h.put)))
	mux.Handle("GET /me/data/{namespace}/{key}", h.auth(http.HandlerFunc(h.get)))
	mux.Handle("DELETE /me/data/{namespace}/{key}", h.auth(http.HandlerFunc(h.del)))
	mux.Handle("GET /me/data/{namespace}", h.auth(http.HandlerFunc(h.getNamespace)))

	mux.HandleFunc("GET /admin/games/{gameId}/players/{playerID}/data", h.adminListForPlayer)
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

func (h *Handler) put(w http.ResponseWriter, r *http.Request) {
	gameID := gameHeader(r)
	if gameID == "" {
		writeError(w, http.StatusBadRequest, "missing_game")
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, "invalid_value")
		return
	}
	if err := h.svc.Put(r.Context(), gameID, playerID(r.Context()), r.PathValue("namespace"), r.PathValue("key"), body); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	gameID := gameHeader(r)
	if gameID == "" {
		writeError(w, http.StatusBadRequest, "missing_game")
		return
	}
	v, err := h.svc.Get(r.Context(), gameID, playerID(r.Context()), r.PathValue("namespace"), r.PathValue("key"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(v)
}

func (h *Handler) getNamespace(w http.ResponseWriter, r *http.Request) {
	gameID := gameHeader(r)
	if gameID == "" {
		writeError(w, http.StatusBadRequest, "missing_game")
		return
	}
	m, err := h.svc.GetNamespace(r.Context(), gameID, playerID(r.Context()), r.PathValue("namespace"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := make(map[string]json.RawMessage, len(m)) // RawMessage: não re-serializa o JSON
	for k, v := range m {
		out[k] = json.RawMessage(v)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) del(w http.ResponseWriter, r *http.Request) {
	gameID := gameHeader(r)
	if gameID == "" {
		writeError(w, http.StatusBadRequest, "missing_game")
		return
	}
	if err := h.svc.Delete(r.Context(), gameID, playerID(r.Context()), r.PathValue("namespace"), r.PathValue("key")); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// adminListForPlayer devolve todo o dado do jogador {playerID} no jogo {gameId}
// (endpoint admin, sem JWT — protegido pela internal key no main). Alimenta a
// aba Player data do painel.
func (h *Handler) adminListForPlayer(w http.ResponseWriter, r *http.Request) {
	entries, err := h.svc.ListForPlayer(r.Context(), r.PathValue("gameId"), r.PathValue("playerID"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": toEntryDTOs(entries)})
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

// entryDTO é a forma de saída do admin read. value sai como JSON cru
// (RawMessage não re-serializa o JSONB do banco).
type entryDTO struct {
	Namespace string          `json:"namespace"`
	Key       string          `json:"key"`
	Value     json.RawMessage `json:"value"`
}

func toEntryDTOs(entries []domain.Entry) []entryDTO {
	out := make([]entryDTO, 0, len(entries))
	for _, e := range entries {
		out = append(out, entryDTO{Namespace: e.Namespace, Key: e.Key, Value: json.RawMessage(e.Value)})
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
	case errors.Is(err, domain.ErrInvalidKey):
		writeError(w, http.StatusBadRequest, "invalid_key")
	case errors.Is(err, domain.ErrInvalidValue):
		writeError(w, http.StatusBadRequest, "invalid_value")
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found")
	default:
		writeError(w, http.StatusInternalServerError, "internal")
	}
}
