// Package transport traduz HTTP para o domínio e vice-versa (handlers do
// game-registry). Sem lógica de negócio. Ver ADR-0002.
package transport

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/vitormartins1/go-game-backend/services/game-registry/internal/domain"
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
	mux.HandleFunc("PUT /games/{id}", h.rename)
	// Perfil de multiplayer (ADR-0023, spec 006).
	mux.HandleFunc("PUT /games/{id}/multiplayer-profile", h.setProfile)
	mux.HandleFunc("GET /games/{id}/multiplayer-profile", h.getProfile)
	// Presença por jogo (ADR-0019): marca (login-into-game) e lista a base.
	mux.HandleFunc("PUT /games/{gameId}/players/{playerID}", h.markPresence)
	mux.HandleFunc("GET /games/{gameId}/players", h.listPlayers)
	// Consulta reversa (004): jogos em que o player está presente.
	mux.HandleFunc("GET /players/{playerID}/games", h.playerGames)
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

// --- perfil de multiplayer (ADR-0023, spec 006) ---

type profileDTO struct {
	Archetype string              `json:"archetype"`
	Topology  string              `json:"topology"`
	Params    queueArenaParamsDTO `json:"params"`
}

type queueArenaParamsDTO struct {
	Structure       string   `json:"structure"`
	PlayerCount     int      `json:"playerCount"`
	TeamCount       int      `json:"teamCount"`
	TeamSize        int      `json:"teamSize"`
	WinCondition    string   `json:"winCondition"`
	ScoreTarget     int      `json:"scoreTarget"`
	DurationSeconds int      `json:"durationSeconds"`
	MatchStructure  string   `json:"matchStructure"`
	RoundsToWin     int      `json:"roundsToWin"`
	AllowDraw       bool     `json:"allowDraw"`
	ScoringStats    []string `json:"scoringStats"`
}

// gameWithProfileDTO estende o jogo com o perfil no GET /games/{id}. O perfil
// serializa como objeto ou null (sem omitempty); a lista usa gameDTO puro.
type gameWithProfileDTO struct {
	gameDTO
	MultiplayerProfile *profileDTO `json:"multiplayerProfile"`
}

func profileToDomain(d profileDTO) domain.MultiplayerProfile {
	p := domain.MultiplayerProfile{Archetype: d.Archetype, Topology: d.Topology}
	if d.Archetype == domain.ArchetypeQueueArena {
		qa := d.Params
		p.QueueArena = &domain.QueueArenaParams{
			Structure: qa.Structure, PlayerCount: qa.PlayerCount, TeamCount: qa.TeamCount,
			TeamSize: qa.TeamSize, WinCondition: qa.WinCondition, ScoreTarget: qa.ScoreTarget,
			DurationSeconds: qa.DurationSeconds, MatchStructure: qa.MatchStructure,
			RoundsToWin: qa.RoundsToWin, AllowDraw: qa.AllowDraw, ScoringStats: qa.ScoringStats,
		}
	}
	return p
}

func profileToDTO(p *domain.MultiplayerProfile) *profileDTO {
	if p == nil {
		return nil
	}
	d := &profileDTO{Archetype: p.Archetype, Topology: p.Topology}
	if p.QueueArena != nil {
		qa := p.QueueArena
		d.Params = queueArenaParamsDTO{
			Structure: qa.Structure, PlayerCount: qa.PlayerCount, TeamCount: qa.TeamCount,
			TeamSize: qa.TeamSize, WinCondition: qa.WinCondition, ScoreTarget: qa.ScoreTarget,
			DurationSeconds: qa.DurationSeconds, MatchStructure: qa.MatchStructure,
			RoundsToWin: qa.RoundsToWin, AllowDraw: qa.AllowDraw, ScoringStats: qa.ScoringStats,
		}
	}
	return d
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
	writeJSON(w, http.StatusOK, gameWithProfileDTO{
		gameDTO:            toDTO(g),
		MultiplayerProfile: profileToDTO(g.MultiplayerProfile),
	})
}

// setProfile faz upsert do perfil de multiplayer do jogo (ADR-0023, spec 006).
func (h *Handler) setProfile(w http.ResponseWriter, r *http.Request) {
	var req profileDTO
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	p, err := h.games.SetMultiplayerProfile(r.Context(), r.PathValue("id"), profileToDomain(req))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, profileToDTO(p))
}

// getProfile devolve o perfil de multiplayer do jogo (404 se ausente).
func (h *Handler) getProfile(w http.ResponseWriter, r *http.Request) {
	p, err := h.games.GetMultiplayerProfile(r.Context(), r.PathValue("id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, profileToDTO(p))
}

// rename altera só o nome do jogo (Configurações do painel, spec 005).
func (h *Handler) rename(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	g, err := h.games.Rename(r.Context(), r.PathValue("id"), req.Name)
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

// playerGames devolve os jogos em que o player {playerID} está presente
// (consulta reversa da presença, feature 004).
func (h *Handler) playerGames(w http.ResponseWriter, r *http.Request) {
	games, err := h.presence.GamesOf(r.Context(), r.PathValue("playerID"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := make([]gameDTO, len(games))
	for i := range games {
		out[i] = toDTO(&games[i])
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
	case errors.Is(err, domain.ErrUnsupportedArchetype):
		writeError(w, http.StatusBadRequest, "unsupported_archetype")
	case errors.Is(err, domain.ErrUnsupportedTopology):
		writeError(w, http.StatusBadRequest, "unsupported_topology")
	case errors.Is(err, domain.ErrInvalidProfileParams):
		writeError(w, http.StatusBadRequest, "invalid_profile_params")
	case errors.Is(err, domain.ErrProfileNotFound):
		writeError(w, http.StatusNotFound, "profile_not_found")
	default:
		writeError(w, http.StatusInternalServerError, "internal")
	}
}
