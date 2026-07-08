// Superfície HTTP admin do session-service (spec 005): a aba Sessions do
// painel lê as partidas de um jogo via BFF. Padrão dos pares (leaderboard):
// /admin sem auth no serviço — internal key na borda (ADR-0018).
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"time"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/domain"
)

// SessionDirectory é o recorte do service que o admin consome (interface do
// consumidor — não polui a Port de entrada usada pelo gRPC). Cobre a aba Sessions
// por-jogo (005) e o board global + force-close do console (007).
type SessionDirectory interface {
	SessionsByGame(gameID string) (active, closed []domain.Session)
	AllSessions() (active, closed []domain.Session)
	CloseByAdmin(ctx context.Context, matchID string) (*domain.Session, error)
}

// AdminHandler expõe as rotas /admin de sessões (leitura + encerrar).
type AdminHandler struct {
	sessions SessionDirectory
}

// NewAdminHandler cria o handler admin.
func NewAdminHandler(sessions SessionDirectory) *AdminHandler {
	return &AdminHandler{sessions: sessions}
}

// Register monta as rotas no mux existente (JWKS/metrics moram lá).
func (h *AdminHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/games/{gameId}/sessions", h.list)
	mux.HandleFunc("GET /admin/sessions", h.listAll)
	mux.HandleFunc("POST /admin/sessions/{matchId}/close", h.close)
}

type sessionDTO struct {
	SessionID     string     `json:"sessionId"`
	MatchID       string     `json:"matchId"`
	GameID        string     `json:"gameId"`
	ModeID        string     `json:"modeId,omitempty"` // modo do jogo (ADR-0024, aditivo)
	State         string     `json:"state"`
	Players       []string   `json:"players"`
	ServerAddress string     `json:"serverAddress"`
	OpenedAt      time.Time  `json:"openedAt"`
	ClosedAt      *time.Time `json:"closedAt,omitempty"`
	CloseReason   string     `json:"closeReason,omitempty"`

	// host_world_dropin (spec 014): origem, host e slots por papel. origin sempre
	// presente; os demais só em mundos host_world (omitidos no queue_arena).
	Origin       string        `json:"origin"`
	HostPlayerID string        `json:"hostPlayerId,omitempty"`
	RoleSlots    []roleSlotDTO `json:"roleSlots,omitempty"`

	// Detalhe rico (spec 018): snapshot do modo (ADR-0024) + formação +
	// resultado persistido. Aditivos; ausentes em sessões sem snapshot/result.
	Archetype   string          `json:"archetype,omitempty"`
	ProfileHash string          `json:"profileHash,omitempty"`
	ModeParams  json.RawMessage `json:"modeParams,omitempty"`
	Teams       []teamDTO       `json:"teams,omitempty"`
	Result      *resultDTO      `json:"result,omitempty"`
}

// teamDTO é um lado da formação registrada no Create (ADR-0023) — o rótulo
// canônico e seus jogadores. Omitido em FFA/host_world (TeamByPlayer vazio).
type teamDTO struct {
	TeamID  string   `json:"teamId"`
	Players []string `json:"players"`
}

// resultDTO espelha o domain.MatchOutcome (tagged union): só o kind reportado
// tem payload. É o que foi publicado no fan-out, com team_id carimbado.
type resultDTO struct {
	Kind        string             `json:"kind"`
	WinLossDraw []playerOutcomeDTO `json:"winLossDraw,omitempty"`
	Placement   []placementDTO     `json:"placement,omitempty"`
	Collective  *collectiveDTO     `json:"collective,omitempty"`
}

type playerOutcomeDTO struct {
	PlayerID string           `json:"playerId"`
	TeamID   string           `json:"teamId,omitempty"`
	Outcome  string           `json:"outcome"`
	Score    int64            `json:"score"`
	Stats    map[string]int64 `json:"stats,omitempty"`
}

type placementDTO struct {
	PlayerID   string           `json:"playerId"`
	TeamID     string           `json:"teamId,omitempty"`
	Placement  int              `json:"placement"`
	SurvivedMs int64            `json:"survivedMs"`
	Stats      map[string]int64 `json:"stats,omitempty"`
}

type collectiveDTO struct {
	Outcome string                `json:"outcome"`
	Players []collectivePlayerDTO `json:"players,omitempty"`
}

type collectivePlayerDTO struct {
	PlayerID string           `json:"playerId"`
	TeamID   string           `json:"teamId,omitempty"`
	Stats    map[string]int64 `json:"stats,omitempty"`
}

// roleSlotDTO é a ocupação de um papel num mundo host_world (co-op X/2, etc.).
type roleSlotDTO struct {
	Role     string `json:"role"`
	Capacity int    `json:"capacity"`
	Occupied int    `json:"occupied"`
}

type sessionsResponse struct {
	Active []sessionDTO `json:"active"`
	Closed []sessionDTO `json:"closed"`
}

// list responde as sessões do jogo. `state=open|closed` filtra; sem sessões =
// 200 com listas vazias (jogo pode existir sem partidas — não é 404).
func (h *AdminHandler) list(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("gameId")
	state := r.URL.Query().Get("state")
	if !validState(state) {
		writeError(w, http.StatusBadRequest, "invalid_state")
		return
	}

	active, closed := h.sessions.SessionsByGame(gameID)
	writeJSON(w, http.StatusOK, filterByState(active, closed, state))
}

// listAll responde as sessões de TODOS os jogos (board global do console, 007).
// `state=open|closed` filtra; sem sessões = 200 com listas vazias.
func (h *AdminHandler) listAll(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	if !validState(state) {
		writeError(w, http.StatusBadRequest, "invalid_state")
		return
	}
	active, closed := h.sessions.AllSessions()
	writeJSON(w, http.StatusOK, filterByState(active, closed, state))
}

// close encerra uma sessão ativa por ação do operador (CLOSED/admin, 007).
func (h *AdminHandler) close(w http.ResponseWriter, r *http.Request) {
	matchID := r.PathValue("matchId")
	sess, err := h.sessions.CloseByAdmin(r.Context(), matchID)
	if errors.Is(err, domain.ErrSessionNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "close_failed")
		return
	}
	writeJSON(w, http.StatusOK, toSessionDTO(*sess))
}

func validState(state string) bool {
	return state == "" || state == string(domain.SessionOpen) || state == string(domain.SessionClosed)
}

// filterByState monta a resposta aplicando o filtro de estado (compartilhado
// entre a listagem por-jogo e a global).
func filterByState(active, closed []domain.Session, state string) sessionsResponse {
	resp := sessionsResponse{Active: []sessionDTO{}, Closed: []sessionDTO{}}
	if state == "" || state == string(domain.SessionOpen) {
		for _, s := range active {
			resp.Active = append(resp.Active, toSessionDTO(s))
		}
	}
	if state == "" || state == string(domain.SessionClosed) {
		for _, s := range closed {
			resp.Closed = append(resp.Closed, toSessionDTO(s))
		}
	}
	return resp
}

func toSessionDTO(s domain.Session) sessionDTO {
	dto := sessionDTO{
		SessionID:     s.ID,
		MatchID:       s.MatchID,
		GameID:        s.GameID,
		ModeID:        s.ModeID,
		State:         string(s.State),
		Players:       s.Players,
		ServerAddress: s.ServerAddress,
		OpenedAt:      s.OpenedAt,
		CloseReason:   string(s.CloseReason),
		Origin:        string(s.Origin),
		HostPlayerID:  s.HostPlayerID,
		RoleSlots:     roleSlotsDTO(s.RoleSlots),
	}
	if !s.ClosedAt.IsZero() {
		closedAt := s.ClosedAt
		dto.ClosedAt = &closedAt
	}
	if s.Profile != nil {
		dto.Archetype = s.Profile.Archetype
		dto.ProfileHash = s.Profile.Hash
		dto.ModeParams = json.RawMessage(s.Profile.ParamsJSON)
	}
	dto.Teams = teamsDTO(s.TeamByPlayer)
	dto.Result = toResultDTO(s.Result)
	return dto
}

// teamsDTO agrupa a formação por time em ordem estável (teamId, players). Nil
// (campo omitido) em FFA/host_world (TeamByPlayer vazio).
func teamsDTO(teamByPlayer map[string]string) []teamDTO {
	if len(teamByPlayer) == 0 {
		return nil
	}
	byTeam := map[string][]string{}
	for player, team := range teamByPlayer {
		byTeam[team] = append(byTeam[team], player)
	}
	out := make([]teamDTO, 0, len(byTeam))
	for team, players := range byTeam {
		sort.Strings(players)
		out = append(out, teamDTO{TeamID: team, Players: players})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TeamID < out[j].TeamID })
	return out
}

func toResultDTO(r *domain.MatchOutcome) *resultDTO {
	if r == nil {
		return nil
	}
	dto := &resultDTO{Kind: r.Kind}
	if r.WinLossDraw != nil {
		dto.WinLossDraw = make([]playerOutcomeDTO, 0, len(r.WinLossDraw.Players))
		for _, p := range r.WinLossDraw.Players {
			dto.WinLossDraw = append(dto.WinLossDraw, playerOutcomeDTO{
				PlayerID: p.PlayerID, TeamID: p.TeamID, Outcome: p.Outcome,
				Score: p.Score, Stats: p.Stats,
			})
		}
	}
	if r.Placement != nil {
		dto.Placement = make([]placementDTO, 0, len(r.Placement.Entries))
		for _, e := range r.Placement.Entries {
			dto.Placement = append(dto.Placement, placementDTO{
				PlayerID: e.PlayerID, TeamID: e.TeamID, Placement: e.Placement,
				SurvivedMs: e.SurvivedMs, Stats: e.Stats,
			})
		}
	}
	if r.Collective != nil {
		players := make([]collectivePlayerDTO, 0, len(r.Collective.Players))
		for _, p := range r.Collective.Players {
			players = append(players, collectivePlayerDTO{PlayerID: p.PlayerID, TeamID: p.TeamID, Stats: p.Stats})
		}
		dto.Collective = &collectiveDTO{Outcome: r.Collective.Outcome, Players: players}
	}
	return dto
}

// roleSlotsDTO serializa os slots por papel em ordem estável (coop, invader) para
// o JSON ser determinístico. Devolve nil (campo omitido) no queue_arena (map vazio).
func roleSlotsDTO(slots map[string]*domain.RoleSlot) []roleSlotDTO {
	if len(slots) == 0 {
		return nil
	}
	out := make([]roleSlotDTO, 0, len(slots))
	for role, slot := range slots {
		out = append(out, roleSlotDTO{Role: role, Capacity: slot.Capacity, Occupied: len(slot.Occupants)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Role < out[j].Role })
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
