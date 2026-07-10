// Package transport traduz HTTP para o domínio e vice-versa (handlers do
// cloud-save-service). Sem lógica de negócio. Ver ADR-0002.
package transport

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/cloud-save-service/internal/domain"
	"github.com/vitormartins1/go-multiplayer-control-plane/shared/auth"
	"github.com/vitormartins1/go-multiplayer-control-plane/shared/httpkit"
)

// maxSaveBytes limita o tamanho do save (proteção; quota por jogador é backlog).
const maxSaveBytes = 5 << 20 // 5 MiB

// Handler expõe os endpoints HTTP do cloud-save-service.
type Handler struct {
	svc      domain.CloudSaveService
	verifier domain.TokenVerifier
}

// NewHandler cria o handler a partir das Ports do domínio.
func NewHandler(svc domain.CloudSaveService, verifier domain.TokenVerifier) *Handler {
	return &Handler{svc: svc, verifier: verifier}
}

type ctxKey int

const playerIDKey ctxKey = iota

// Register monta as rotas. As de jogador ficam atrás do auth (JWT) e pegam o
// jogo do header X-Game-Id; a admin (jogo/jogador no path) fica fora do JWT —
// protegida pela internal key no main (ADR-0018).
func (h *Handler) Register(mux *http.ServeMux) {
	mux.Handle("PUT /me/save", h.auth(http.HandlerFunc(h.save)))
	mux.Handle("GET /me/save", h.auth(http.HandlerFunc(h.load)))

	mux.HandleFunc("GET /admin/games/{gameId}/players/{playerID}/save", h.adminLoad)
}

// auth valida o Bearer JWT (via TokenVerifier) e injeta o playerID no contexto.
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

func (h *Handler) save(w http.ResponseWriter, r *http.Request) {
	gameID := gameHeader(r)
	if gameID == "" {
		httpkit.WriteError(w, http.StatusBadRequest, "missing_game")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxSaveBytes)
	blob, err := io.ReadAll(r.Body)
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			httpkit.WriteError(w, http.StatusRequestEntityTooLarge, "too_large")
			return
		}
		httpkit.WriteError(w, http.StatusBadRequest, "bad_request")
		return
	}
	ct := r.Header.Get("Content-Type")
	if ct == "" {
		ct = "application/octet-stream"
	}
	if err := h.svc.Save(r.Context(), gameID, playerID(r.Context()), domain.Snapshot{Blob: blob, ContentType: ct}); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) load(w http.ResponseWriter, r *http.Request) {
	gameID := gameHeader(r)
	if gameID == "" {
		httpkit.WriteError(w, http.StatusBadRequest, "missing_game")
		return
	}
	snap, err := h.svc.Load(r.Context(), gameID, playerID(r.Context()))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	w.Header().Set("Content-Type", snap.ContentType)
	_, _ = w.Write(snap.Blob)
}

// adminSaveDTO é a forma de saída do admin read: metadados + conteúdo em base64
// (o blob é binário; base64 é seguro em JSON). Alimenta a aba Cloud saves.
type adminSaveDTO struct {
	ContentType string    `json:"contentType"`
	Size        int       `json:"size"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Blob        string    `json:"blob"` // base64 (std)
}

// adminLoad devolve o save do jogador {playerID} no jogo {gameId} (endpoint
// admin, sem JWT — protegido pela internal key no main). 404 se não existir.
func (h *Handler) adminLoad(w http.ResponseWriter, r *http.Request) {
	snap, err := h.svc.LoadFor(r.Context(), r.PathValue("gameId"), r.PathValue("playerID"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	httpkit.WriteJSON(w, http.StatusOK, adminSaveDTO{
		ContentType: snap.ContentType,
		Size:        len(snap.Blob),
		UpdatedAt:   snap.UpdatedAt,
		Blob:        base64.StdEncoding.EncodeToString(snap.Blob),
	})
}

// --- helpers ---

// gameHeader lê o jogo do header X-Game-Id (contexto ambiente do data plane).
func gameHeader(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get("X-Game-Id"))
}

func playerID(ctx context.Context) string {
	id, _ := ctx.Value(playerIDKey).(string)
	return id
}

// writeDomainError mapeia erros de domínio para HTTP status + código estável.
func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		httpkit.WriteError(w, http.StatusNotFound, "not_found")
	default:
		httpkit.WriteError(w, http.StatusInternalServerError, "internal")
	}
}
