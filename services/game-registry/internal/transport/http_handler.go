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
// (consumido pelo painel admin e por serviços), atrás da internal key (ADR-0018).
type Handler struct {
	games    domain.GameService
	presence domain.PresenceService
}

// NewHandler cria o handler a partir das Ports de entrada.
func NewHandler(games domain.GameService, presence domain.PresenceService) *Handler {
	return &Handler{games: games, presence: presence}
}

// Register monta as rotas no mux fornecido.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /games", h.create)
	mux.HandleFunc("GET /games", h.list)
	mux.HandleFunc("GET /games/{id}", h.get)
	// Presença por jogo (ADR-0019): marca (login-into-game) e lista a base.
	mux.HandleFunc("PUT /games/{gameId}/players/{playerID}", h.markPresence)
	mux.HandleFunc("GET /games/{gameId}/players", h.listPlayers)
}

type gameDTO struct {
	ID        string    `json:"id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	// Capa (003-generated-cover): metadados; a imagem é gerada no cliente.
	Image               string `json:"image"`               // "" = sem upload (reservado)
	ImageSource         string `json:"imageSource"`         // "generated" no MVP
	GeneratedImageSeed  string `json:"generatedImageSeed"`  // default = slug
	GeneratedImageStyle string `json:"generatedImageStyle"` // default = "shapes"
}

func toDTO(g *domain.Game) gameDTO {
	return gameDTO{
		ID: g.ID, Slug: g.Slug, Name: g.Name, CreatedAt: g.CreatedAt,
		Image:               g.Image,
		ImageSource:         g.ImageSource,
		GeneratedImageSeed:  g.GeneratedImageSeed,
		GeneratedImageStyle: g.GeneratedImageStyle,
	}
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

// markPresence registra a presença do jogador {playerID} no jogo {gameId}
// (upsert idempotente). Chamado pelo identity no login-into-game.
func (h *Handler) markPresence(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Handle string `json:"handle"`
	}
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	if err := h.presence.Mark(r.Context(), r.PathValue("gameId"), r.PathValue("playerID"), req.Handle); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type presenceDTO struct {
	PlayerID  string    `json:"playerId"`
	Handle    string    `json:"handle"`
	FirstSeen time.Time `json:"firstSeen"`
}

// listPlayers devolve a player base do jogo {gameId} (presença).
func (h *Handler) listPlayers(w http.ResponseWriter, r *http.Request) {
	players, err := h.presence.PlayersOf(r.Context(), r.PathValue("gameId"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := make([]presenceDTO, len(players))
	for i, p := range players {
		out[i] = presenceDTO{PlayerID: p.PlayerID, Handle: p.Handle, FirstSeen: p.FirstSeen}
	}
	writeJSON(w, http.StatusOK, out)
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
