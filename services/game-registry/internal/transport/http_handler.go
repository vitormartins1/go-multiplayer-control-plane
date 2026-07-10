// Package transport traduz HTTP para o domínio e vice-versa (handlers do
// game-registry). Sem lógica de negócio. Ver ADR-0002.
package transport

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/game-registry/internal/domain"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/game-registry/internal/jsonparams"
)

// hostWorldDropInKnownKeys é o vocabulário tipado do arquétipo (tags JSON de
// hostWorldDropInParamsDTO) — qualquer outra chave em `params` é preservada
// crua via jsonparams (follow-up #14, MultiplayerProfile.ExtraParams).
var hostWorldDropInKnownKeys = map[string]struct{}{
	"enabledRoles":      {},
	"coopSlots":         {},
	"invaderSlots":      {},
	"levelRangeEnabled": {},
	"weaponTierEnabled": {},
	"passwordEnabled":   {},
	"invasionEnabled":   {},
	"scoringStats":      {},
}

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
	mux.HandleFunc("DELETE /games/{id}", h.delete)
	// Fleet Agones (ADR-0026, spec 012).
	mux.HandleFunc("PUT /games/{id}/fleet", h.setFleet)
	// Perfil de multiplayer — alias legado do modo "default" (ADR-0023/0024).
	mux.HandleFunc("PUT /games/{id}/multiplayer-profile", h.setProfile)
	mux.HandleFunc("GET /games/{id}/multiplayer-profile", h.getProfile)
	// Modos nomeados (ADR-0024, spec 009).
	mux.HandleFunc("GET /games/{id}/multiplayer-profiles", h.listModes)
	mux.HandleFunc("PUT /games/{id}/multiplayer-profiles/{modeId}", h.setMode)
	mux.HandleFunc("GET /games/{id}/multiplayer-profiles/{modeId}", h.getMode)
	mux.HandleFunc("DELETE /games/{id}/multiplayer-profiles/{modeId}", h.deleteMode)
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
	// Fleet Agones do jogo (ADR-0026, spec 012); "" = sem Agones real habilitado.
	Fleet string `json:"fleet"`
}

func toDTO(g *domain.Game) gameDTO {
	return gameDTO{
		ID: g.ID, Slug: g.Slug, Name: g.Name, CreatedAt: g.CreatedAt,
		Image:               g.Image,
		ImageSource:         g.ImageSource,
		GeneratedImageSeed:  g.GeneratedImageSeed,
		GeneratedImageStyle: g.GeneratedImageStyle,
		Fleet:               g.Fleet,
	}
}

// --- perfil de multiplayer (ADR-0023) ---

// profileDTO é o envelope de serialização HTTP. Params é raw para suportar
// vários arquétipos sem ter que deserializar e re-serializar o corpo.
type profileDTO struct {
	Archetype string          `json:"archetype"`
	Topology  string          `json:"topology"`
	Params    json.RawMessage `json:"params"`
}

type queueArenaParamsDTO struct {
	Structure       string   `json:"structure"`
	PlayerCount     int      `json:"playerCount,omitempty"`
	TeamCount       int      `json:"teamCount,omitempty"`
	TeamSize        int      `json:"teamSize,omitempty"`
	WinCondition    string   `json:"winCondition"`
	ScoreTarget     int      `json:"scoreTarget,omitempty"`
	DurationSeconds int      `json:"durationSeconds,omitempty"`
	MatchStructure  string   `json:"matchStructure"`
	RoundsToWin     int      `json:"roundsToWin,omitempty"`
	AllowDraw       bool     `json:"allowDraw"`
	ScoringStats    []string `json:"scoringStats"`
}

type hostWorldDropInParamsDTO struct {
	EnabledRoles      []string `json:"enabledRoles"`
	CoopSlots         int      `json:"coopSlots,omitempty"`
	InvaderSlots      int      `json:"invaderSlots,omitempty"`
	LevelRangeEnabled bool     `json:"levelRangeEnabled"`
	WeaponTierEnabled bool     `json:"weaponTierEnabled"`
	PasswordEnabled   bool     `json:"passwordEnabled"`
	InvasionEnabled   bool     `json:"invasionEnabled"`
	ScoringStats      []string `json:"scoringStats"`
}

type battleRoyaleParamsDTO struct {
	LobbySize    int      `json:"lobbySize,omitempty"`
	TeamSize     int      `json:"teamSize,omitempty"`
	RespawnMode  string   `json:"respawnMode"`
	ScoringStats []string `json:"scoringStats"`
}

type coopMissionParamsDTO struct {
	TeamSize       int      `json:"teamSize,omitempty"`
	Missions       []string `json:"missions"`
	Difficulties   []string `json:"difficulties"`
	BotfillEnabled bool     `json:"botfillEnabled"`
	ScoringStats   []string `json:"scoringStats"`
}

// gameWithProfileDTO estende o jogo com o perfil (modo default, compat) e o
// resumo dos modos no GET /games/{id}. O perfil serializa como objeto ou null
// (sem omitempty); a lista usa gameDTO puro.
type gameWithProfileDTO struct {
	gameDTO
	MultiplayerProfile *profileDTO      `json:"multiplayerProfile"`
	MultiplayerModes   []modeSummaryDTO `json:"multiplayerModes"`
}

// modeSummaryDTO é o resumo de um modo no GET /games/{id} (spec 009).
type modeSummaryDTO struct {
	ModeID      string `json:"modeId"`
	DisplayName string `json:"displayName,omitempty"`
	Archetype   string `json:"archetype"`
}

// modeDTO é um modo completo (coleção multiplayer-profiles). O embed do
// profileDTO achata archetype/topology/params no mesmo objeto JSON. Fleet é
// atributo do JOGO dono do modo (ADR-0026), não do perfil — por isso fora do
// embed; omitempty porque nem todo call-site tem o valor à mão (listModes/
// setMode devolvem "").
type modeDTO struct {
	ModeID      string `json:"modeId"`
	DisplayName string `json:"displayName,omitempty"`
	Fleet       string `json:"fleet,omitempty"`
	profileDTO
}

// setModeRequest é o corpo do PUT do modo (modeId vem do path).
type setModeRequest struct {
	DisplayName string          `json:"displayName"`
	Archetype   string          `json:"archetype"`
	Topology    string          `json:"topology"`
	Params      json.RawMessage `json:"params"`
}

func toModeDTO(m *domain.MultiplayerMode, fleet string) modeDTO {
	return modeDTO{ModeID: m.ModeID, DisplayName: m.DisplayName, Fleet: fleet, profileDTO: *profileToDTO(&m.Profile)}
}

func profileToDomain(d profileDTO) domain.MultiplayerProfile {
	p := domain.MultiplayerProfile{Archetype: d.Archetype, Topology: d.Topology}
	switch d.Archetype {
	case domain.ArchetypeQueueArena:
		var qa queueArenaParamsDTO
		_ = json.Unmarshal(d.Params, &qa)
		p.QueueArena = &domain.QueueArenaParams{
			Structure: qa.Structure, PlayerCount: qa.PlayerCount, TeamCount: qa.TeamCount,
			TeamSize: qa.TeamSize, WinCondition: qa.WinCondition, ScoreTarget: qa.ScoreTarget,
			DurationSeconds: qa.DurationSeconds, MatchStructure: qa.MatchStructure,
			RoundsToWin: qa.RoundsToWin, AllowDraw: qa.AllowDraw, ScoringStats: nonNil(qa.ScoringStats),
		}
	case domain.ArchetypeHostWorldDropIn:
		var hw hostWorldDropInParamsDTO
		_ = json.Unmarshal(d.Params, &hw)
		p.HostWorldDropIn = &domain.HostWorldDropInParams{
			EnabledRoles:      nonNil(hw.EnabledRoles),
			CoopSlots:         hw.CoopSlots,
			InvaderSlots:      hw.InvaderSlots,
			LevelRangeEnabled: hw.LevelRangeEnabled,
			WeaponTierEnabled: hw.WeaponTierEnabled,
			PasswordEnabled:   hw.PasswordEnabled,
			InvasionEnabled:   hw.InvasionEnabled,
			ScoringStats:      nonNil(hw.ScoringStats),
		}
		p.ExtraParams = jsonparams.Extract(d.Params, hostWorldDropInKnownKeys)
	case domain.ArchetypeBattleRoyale:
		var br battleRoyaleParamsDTO
		_ = json.Unmarshal(d.Params, &br)
		p.BattleRoyale = &domain.BattleRoyaleParams{
			LobbySize:    br.LobbySize,
			TeamSize:     br.TeamSize,
			RespawnMode:  br.RespawnMode,
			ScoringStats: nonNil(br.ScoringStats),
		}
	case domain.ArchetypeCoopMission:
		var cm coopMissionParamsDTO
		_ = json.Unmarshal(d.Params, &cm)
		p.CoopMission = &domain.CoopMissionParams{
			TeamSize:       cm.TeamSize,
			Missions:       cm.Missions,
			Difficulties:   cm.Difficulties,
			BotfillEnabled: cm.BotfillEnabled,
			ScoringStats:   cm.ScoringStats,
		}
	}
	return p
}

// nonNil garante slice não-nil na serialização: o contrato do console declara
// esses campos como array obrigatório — lista vazia vai como [], nunca null/
// ausente (bug: params sem scoringStats quebrava a UI).
func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func profileToDTO(p *domain.MultiplayerProfile) *profileDTO {
	if p == nil {
		return nil
	}
	d := &profileDTO{Archetype: p.Archetype, Topology: p.Topology}
	switch p.Archetype {
	case domain.ArchetypeQueueArena:
		if p.QueueArena != nil {
			qa := p.QueueArena
			raw, _ := json.Marshal(queueArenaParamsDTO{
				Structure: qa.Structure, PlayerCount: qa.PlayerCount, TeamCount: qa.TeamCount,
				TeamSize: qa.TeamSize, WinCondition: qa.WinCondition, ScoreTarget: qa.ScoreTarget,
				DurationSeconds: qa.DurationSeconds, MatchStructure: qa.MatchStructure,
				RoundsToWin: qa.RoundsToWin, AllowDraw: qa.AllowDraw, ScoringStats: nonNil(qa.ScoringStats),
			})
			d.Params = raw
		}
	case domain.ArchetypeHostWorldDropIn:
		if p.HostWorldDropIn != nil {
			hw := p.HostWorldDropIn
			raw, _ := json.Marshal(hostWorldDropInParamsDTO{
				EnabledRoles:      nonNil(hw.EnabledRoles),
				CoopSlots:         hw.CoopSlots,
				InvaderSlots:      hw.InvaderSlots,
				LevelRangeEnabled: hw.LevelRangeEnabled,
				WeaponTierEnabled: hw.WeaponTierEnabled,
				PasswordEnabled:   hw.PasswordEnabled,
				InvasionEnabled:   hw.InvasionEnabled,
				ScoringStats:      nonNil(hw.ScoringStats),
			})
			d.Params = jsonparams.Merge(raw, p.ExtraParams)
		}
	case domain.ArchetypeBattleRoyale:
		if p.BattleRoyale != nil {
			br := p.BattleRoyale
			raw, _ := json.Marshal(battleRoyaleParamsDTO{
				LobbySize:    br.LobbySize,
				TeamSize:     br.TeamSize,
				RespawnMode:  br.RespawnMode,
				ScoringStats: nonNil(br.ScoringStats),
			})
			d.Params = raw
		}
	case domain.ArchetypeCoopMission:
		if p.CoopMission != nil {
			cm := p.CoopMission
			raw, _ := json.Marshal(coopMissionParamsDTO{
				TeamSize:       cm.TeamSize,
				Missions:       nonNil(cm.Missions),
				Difficulties:   nonNil(cm.Difficulties),
				BotfillEnabled: cm.BotfillEnabled,
				ScoringStats:   nonNil(cm.ScoringStats),
			})
			d.Params = raw
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
	summaries := make([]modeSummaryDTO, 0, len(g.MultiplayerModes))
	for i := range g.MultiplayerModes {
		m := &g.MultiplayerModes[i]
		summaries = append(summaries, modeSummaryDTO{
			ModeID: m.ModeID, DisplayName: m.DisplayName, Archetype: m.Profile.Archetype,
		})
	}
	writeJSON(w, http.StatusOK, gameWithProfileDTO{
		gameDTO:            toDTO(g),
		MultiplayerProfile: profileToDTO(g.MultiplayerProfile),
		MultiplayerModes:   summaries,
	})
}

// --- modos nomeados (ADR-0024, spec 009) ---

func (h *Handler) listModes(w http.ResponseWriter, r *http.Request) {
	modes, err := h.games.ListModes(r.Context(), r.PathValue("id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := make([]modeDTO, 0, len(modes))
	for i := range modes {
		out = append(out, toModeDTO(&modes[i], ""))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) setMode(w http.ResponseWriter, r *http.Request) {
	var req setModeRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	p := profileToDomain(profileDTO{Archetype: req.Archetype, Topology: req.Topology, Params: req.Params})
	m, err := h.games.SetMode(r.Context(), r.PathValue("id"), r.PathValue("modeId"), req.DisplayName, p)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toModeDTO(m, ""))
}

// getMode inclui a Fleet do jogo na resposta (ADR-0026, spec 012) — é o
// endpoint que o session-service (registryclient) consome para montar o
// ProfileSnapshot numa única chamada.
func (h *Handler) getMode(w http.ResponseWriter, r *http.Request) {
	m, fleet, err := h.games.GetMode(r.Context(), r.PathValue("id"), r.PathValue("modeId"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toModeDTO(m, fleet))
}

func (h *Handler) deleteMode(w http.ResponseWriter, r *http.Request) {
	if err := h.games.DeleteMode(r.Context(), r.PathValue("id"), r.PathValue("modeId")); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

// setFleet atualiza a Fleet Agones do jogo (ADR-0026, spec 012).
func (h *Handler) setFleet(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Fleet string `json:"fleet"`
	}
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	g, err := h.games.SetFleet(r.Context(), r.PathValue("id"), req.Fleet)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toDTO(g))
}

// delete marca o jogo como removido (soft delete, ADR-0025, spec 010): 204;
// inexistente ou já deletado -> 404.
func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.games.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
	case errors.Is(err, domain.ErrInvalidFleet):
		writeError(w, http.StatusBadRequest, "invalid_fleet")
	case errors.Is(err, domain.ErrUnsupportedArchetype):
		writeError(w, http.StatusBadRequest, "unsupported_archetype")
	case errors.Is(err, domain.ErrUnsupportedTopology):
		writeError(w, http.StatusBadRequest, "unsupported_topology")
	case errors.Is(err, domain.ErrInvalidProfileParams):
		writeError(w, http.StatusBadRequest, "invalid_profile_params")
	case errors.Is(err, domain.ErrProfileNotFound):
		writeError(w, http.StatusNotFound, "profile_not_found")
	case errors.Is(err, domain.ErrInvalidModeID):
		writeError(w, http.StatusBadRequest, "invalid_mode_id")
	case errors.Is(err, domain.ErrModeNotFound):
		writeError(w, http.StatusNotFound, "mode_not_found")
	case errors.Is(err, domain.ErrDefaultModeProtected):
		writeError(w, http.StatusConflict, "default_mode_protected")
	case errors.Is(err, domain.ErrTooManyModes):
		writeError(w, http.StatusBadRequest, "too_many_modes")
	default:
		writeError(w, http.StatusInternalServerError, "internal")
	}
}
