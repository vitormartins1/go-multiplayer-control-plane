// Package transport traduz HTTP para o domínio e vice-versa (handlers do
// game-registry). Sem lógica de negócio. Ver ADR-0002.
package transport

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/c130n/game-backend/services/game-registry/internal/domain"
)

// Handler expõe os endpoints HTTP do game-registry. É control plane interno
// (consumido pelo painel admin e por serviços) — sem auth no MVP.
type Handler struct {
	games domain.GameService
}

// NewHandler cria o handler a partir da Port de entrada.
func NewHandler(games domain.GameService) *Handler {
	return &Handler{games: games}
}

// Register monta as rotas no mux fornecido.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /games", h.create)
	mux.HandleFunc("GET /games", h.list)
	mux.HandleFunc("GET /games/{id}", h.get)
}

type gameDTO struct {
	ID        string    `json:"id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

func toDTO(g *domain.Game) gameDTO {
	return gameDTO{ID: g.ID, Slug: g.Slug, Name: g.Name, CreatedAt: g.CreatedAt}
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Slug string `json:"slug"`
		Name string `json:"name"`
	}
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	g, err := h.games.Register(r.Context(), req.Slug, req.Name)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toDTO(g))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	games, err := h.games.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	out := make([]gameDTO, len(games))
	for i := range games {
		out[i] = toDTO(&games[i])
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	g, err := h.games.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toDTO(g))
}

// --- helpers ---

func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
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
	case errors.Is(err, domain.ErrInvalidSlug):
		writeError(w, http.StatusBadRequest, "invalid_slug")
	case errors.Is(err, domain.ErrInvalidName):
		writeError(w, http.StatusBadRequest, "invalid_name")
	case errors.Is(err, domain.ErrSlugTaken):
		writeError(w, http.StatusConflict, "slug_taken")
	case errors.Is(err, domain.ErrGameNotFound):
		writeError(w, http.StatusNotFound, "not_found")
	default:
		writeError(w, http.StatusInternalServerError, "internal")
	}
}
