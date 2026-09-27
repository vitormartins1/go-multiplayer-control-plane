// Superfície HTTP admin do session-service (spec 005): a aba Sessions do
// painel lê as partidas de um jogo via BFF. Padrão dos pares (leaderboard):
// /admin sem auth no serviço — internal key na borda (ADR-0018).
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/domain"
	"github.com/vitormartins1/go-multiplayer-control-plane/shared/httpkit"
)

// SessionDirectory é o recorte do service que o admin consome (interface do
// consumidor — não polui a Port de entrada usada pelo gRPC). Cobre a aba Sessions
// por-jogo (005) e o board global + force-close do console (007).
type SessionDirectory interface {
	SessionsByGame(gameID string) (active, closed []domain.Session)
	AllSessions() (active, closed []domain.Session)
	CloseByAdmin(ctx context.Context, matchID string) (*domain.Session, error)
}

// HistoryReader é o recorte de domain.HistoryStore que o admin consome (spec
// 025): leitura paginada das sessões fechadas PERSISTIDAS — irmão de
// SessionDirectory, que cobre só o estado em memória (ativas + ring bounded).
type HistoryReader interface {
	ClosedByGame(ctx context.Context, gameID string, limit int, before time.Time) ([]domain.Session, error)
	ClosedByPlayer(ctx context.Context, gameID, playerID string, limit int, before time.Time) ([]domain.Session, error)
}

// AdminHandler expõe as rotas /admin de sessões (leitura + encerrar) e de
// status de Fleet (spec 023).
type AdminHandler struct {
	sessions SessionDirectory
	// history é o store persistido de fechadas (spec 025). nil = sem Postgres
	// configurado — as rotas paginadas respondem 503 history_unavailable.
	history HistoryReader
	// fleets e clusterLabel cobrem a spec 023 (status real da Fleet Agones).
	// fleets nil = allocator não é agones ("sem cluster observável"); mesmo
	// padrão opcional do ServerRegistry.
	fleets       domain.FleetInspector
	clusterLabel string
}

// NewAdminHandler cria o handler admin. fleets/clusterLabel vêm vazios
// (fleets nil, clusterLabel "") quando o allocator ativo não é o Agones;
// history vem nil quando o session-service sobe sem Postgres configurado.
func NewAdminHandler(sessions SessionDirectory, history HistoryReader, fleets domain.FleetInspector, clusterLabel string) *AdminHandler {
	return &AdminHandler{sessions: sessions, history: history, fleets: fleets, clusterLabel: clusterLabel}
}

// Register monta as rotas no mux existente (JWKS/metrics moram lá).
func (h *AdminHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/games/{gameId}/sessions", h.list)
	mux.HandleFunc("GET /admin/sessions", h.listAll)
	mux.HandleFunc("POST /admin/sessions/{matchId}/close", h.close)
	mux.HandleFunc("GET /admin/fleet-status/{fleetName}", h.fleetStatus)
	// histórico persistido por jogador (spec 025) — a aba History do player.
	mux.HandleFunc("GET /admin/players/{playerId}/sessions", h.playerSessions)
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
	Archetype string `json:"archetype,omitempty"`
	Topology  string `json:"topology,omitempty"` // dedicated | p2p (spec 031); vazio em sessões fechadas antes da migration 004
	// HostConnection (p2p, spec 031) e LastHeartbeatAt (spec 032): o console
	// mostra com quem o jogo conecta e se o mundo está vivo (admin-ui/091).
	HostConnection  string          `json:"hostConnection,omitempty"`
	LastHeartbeatAt *time.Time      `json:"lastHeartbeatAt,omitempty"`
	ProfileHash     string          `json:"profileHash,omitempty"`
	ModeParams      json.RawMessage `json:"modeParams,omitempty"`
	Teams           []teamDTO       `json:"teams,omitempty"`
	Result          *resultDTO      `json:"result,omitempty"`

	// Participants é a base do histórico persistido (spec 025): um registro por
	// jogador com o papel ocupado. Aditivo — vem preenchido tanto nas sessões em
	// memória quanto nas lidas do store persistido (que não populam RoleSlots).
	Participants []participantDTO `json:"participants,omitempty"`
}

// participantDTO espelha domain.Participant (spec 025). LastEvent/LastOutcome
// (spec 027) só existem em host_world_dropin — omitidos quando vazios.
type participantDTO struct {
	PlayerID    string    `json:"playerId"`
	Role        string    `json:"role,omitempty"`
	JoinedAt    time.Time `json:"joinedAt"`
	LastEvent   string    `json:"lastEvent,omitempty"`
	LastOutcome string    `json:"lastOutcome,omitempty"`
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
// Occupants (spec 019) diz QUEM está em cada papel — o console monta os cards
// por papel com os jogadores atribuídos, não só a contagem.
type roleSlotDTO struct {
	Role      string   `json:"role"`
	Capacity  int      `json:"capacity"`
	Occupied  int      `json:"occupied"`
	Occupants []string `json:"occupants,omitempty"`
}

type sessionsResponse struct {
	Active []sessionDTO `json:"active"`
	Closed []sessionDTO `json:"closed"`
}

// list responde as sessões do jogo. `state=open|closed` filtra; sem sessões =
// 200 com listas vazias (jogo pode existir sem partidas — não é 404).
//
// `state=closed` muda de fonte (spec 025): em vez do ring in-memory (só as
// últimas ~200 fechadas de TODOS os jogos), lê o store persistido, paginado
// por closed_at (`limit`/`before`) — sobrevive a restart. Os demais valores de
// `state` (vazio/"open") mantêm o comportamento anterior, inalterado.
func (h *AdminHandler) list(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("gameId")
	state := r.URL.Query().Get("state")
	if !validState(state) {
		httpkit.WriteError(w, http.StatusBadRequest, "invalid_state")
		return
	}
	if state == string(domain.SessionClosed) {
		h.closedFromStore(w, r, func(ctx context.Context, limit int, before time.Time) ([]domain.Session, error) {
			return h.history.ClosedByGame(ctx, gameID, limit, before)
		})
		return
	}

	active, closed := h.sessions.SessionsByGame(gameID)
	httpkit.WriteJSON(w, http.StatusOK, filterByState(active, closed, state))
}

// playerSessions responde as sessões fechadas do jogo em que o player
// participou (spec 025, aba History do player), paginadas por closed_at.
// `gameId` é obrigatório — o histórico é sempre escopado a um jogo (a página
// do player já é escopada por jogo via presença, ADR-0019).
func (h *AdminHandler) playerSessions(w http.ResponseWriter, r *http.Request) {
	playerID := r.PathValue("playerId")
	gameID := r.URL.Query().Get("gameId")
	if gameID == "" {
		httpkit.WriteError(w, http.StatusBadRequest, "missing_game_id")
		return
	}
	h.closedFromStore(w, r, func(ctx context.Context, limit int, before time.Time) ([]domain.Session, error) {
		return h.history.ClosedByPlayer(ctx, gameID, playerID, limit, before)
	})
}

// closedSessionsPage é a resposta paginada do store persistido (spec 025):
// NextBefore é o cursor da próxima página (ausente = não há mais páginas).
type closedSessionsPage struct {
	Closed     []sessionDTO `json:"closed"`
	NextBefore *time.Time   `json:"nextBefore,omitempty"`
}

// closedFromStore monta a página de fechadas via fetch (ClosedByGame ou
// ClosedByPlayer), com o cursor de paginação. history nil (Postgres não
// configurado) ou erro na leitura (Postgres fora, plan D6) -> 503
// history_unavailable; o client cai pro comportamento atual (ring) com aviso.
func (h *AdminHandler) closedFromStore(w http.ResponseWriter, r *http.Request, fetch func(ctx context.Context, limit int, before time.Time) ([]domain.Session, error)) {
	if h.history == nil {
		httpkit.WriteError(w, http.StatusServiceUnavailable, "history_unavailable")
		return
	}
	limit, before, err := parsePagination(r)
	if err != nil {
		httpkit.WriteError(w, http.StatusBadRequest, "invalid_pagination")
		return
	}
	// pede limit+1: se vier um item a mais, sabemos que há próxima página sem
	// precisar de um COUNT(*) separado.
	sessions, err := fetch(r.Context(), limit+1, before)
	if err != nil {
		httpkit.WriteError(w, http.StatusServiceUnavailable, "history_unavailable")
		return
	}
	hasMore := len(sessions) > limit
	if hasMore {
		sessions = sessions[:limit]
	}
	page := closedSessionsPage{Closed: make([]sessionDTO, 0, len(sessions))}
	for _, s := range sessions {
		page.Closed = append(page.Closed, toSessionDTO(s))
	}
	if hasMore {
		next := sessions[len(sessions)-1].ClosedAt
		page.NextBefore = &next
	}
	httpkit.WriteJSON(w, http.StatusOK, page)
}

// parsePagination lê limit (default 50, máx 200) e before (RFC3339; vazio =
// sem cursor, primeira página) da query (spec 025).
func parsePagination(r *http.Request) (limit int, before time.Time, err error) {
	const defaultLimit, maxLimit = 50, 200
	limit = defaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, convErr := strconv.Atoi(raw)
		if convErr != nil || n <= 0 {
			return 0, time.Time{}, fmt.Errorf("limit inválido: %q", raw)
		}
		limit = min(n, maxLimit)
	}
	if raw := r.URL.Query().Get("before"); raw != "" {
		before, err = time.Parse(time.RFC3339, raw)
		if err != nil {
			return 0, time.Time{}, fmt.Errorf("before inválido: %q", raw)
		}
	}
	return limit, before, nil
}

// listAll responde as sessões de TODOS os jogos (board global do console, 007).
// `state=open|closed` filtra; sem sessões = 200 com listas vazias.
func (h *AdminHandler) listAll(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	if !validState(state) {
		httpkit.WriteError(w, http.StatusBadRequest, "invalid_state")
		return
	}
	active, closed := h.sessions.AllSessions()
	httpkit.WriteJSON(w, http.StatusOK, filterByState(active, closed, state))
}

// close encerra uma sessão ativa por ação do operador (CLOSED/admin, 007).
func (h *AdminHandler) close(w http.ResponseWriter, r *http.Request) {
	matchID := r.PathValue("matchId")
	sess, err := h.sessions.CloseByAdmin(r.Context(), matchID)
	if errors.Is(err, domain.ErrSessionNotFound) {
		httpkit.WriteError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		httpkit.WriteError(w, http.StatusInternalServerError, "close_failed")
		return
	}
	httpkit.WriteJSON(w, http.StatusOK, toSessionDTO(*sess))
}

// fleetStatusDTO é o shape de GET /admin/fleet-status/{fleetName} (spec 023).
type fleetStatusDTO struct {
	Cluster     string `json:"cluster"`
	FleetExists bool   `json:"fleetExists"`
	Desired     int    `json:"desired"`
	Current     int    `json:"current"`
	Ready       int    `json:"ready"`
	Allocated   int    `json:"allocated"`
}

// fleetStatus lê o estado real da Fleet pelo nome (spec 023). fleets nil
// (allocator != agones) devolve o shape zerado com cluster="" — sem chamada
// nenhuma; é o caso 2 da spec, distinto do caso 3 (Agones ativo, Fleet
// inexistente NESTE cluster), que preenche Cluster mesmo com FleetExists=false.
func (h *AdminHandler) fleetStatus(w http.ResponseWriter, r *http.Request) {
	fleetName := r.PathValue("fleetName")
	if h.fleets == nil {
		httpkit.WriteJSON(w, http.StatusOK, fleetStatusDTO{Cluster: ""})
		return
	}
	status, err := h.fleets.FleetStatus(r.Context(), fleetName)
	if err != nil {
		httpkit.WriteError(w, http.StatusInternalServerError, "fleet_status_failed")
		return
	}
	httpkit.WriteJSON(w, http.StatusOK, fleetStatusDTO{
		Cluster:     h.clusterLabel,
		FleetExists: status.FleetExists,
		Desired:     status.Desired,
		Current:     status.Current,
		Ready:       status.Ready,
		Allocated:   status.Allocated,
	})
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
	players := s.Players
	if len(players) == 0 && len(s.Participants) > 0 {
		// sessões lidas do store persistido (spec 025) não populam Players —
		// deriva de Participants para não quebrar quem lê o campo legado.
		players = playerIDs(s.Participants)
	}
	dto := sessionDTO{
		SessionID:     s.ID,
		MatchID:       s.MatchID,
		GameID:        s.GameID,
		ModeID:        s.ModeID,
		State:         string(s.State),
		Players:       players,
		ServerAddress: s.ServerAddress,
		OpenedAt:      s.OpenedAt,
		CloseReason:   string(s.CloseReason),
		Origin:        string(s.Origin),
		HostPlayerID:  s.HostPlayerID,
		RoleSlots:     roleSlotsDTO(s.RoleSlots),
		Participants:  participantsDTO(s.Participants),
	}
	dto.HostConnection = s.HostConnection
	if !s.LastHeartbeatAt.IsZero() {
		hb := s.LastHeartbeatAt
		dto.LastHeartbeatAt = &hb
	}
	if !s.ClosedAt.IsZero() {
		closedAt := s.ClosedAt
		dto.ClosedAt = &closedAt
	}
	if s.Profile != nil {
		dto.Archetype = s.Profile.Archetype
		dto.Topology = s.Profile.Topology
		dto.ProfileHash = s.Profile.Hash
		dto.ModeParams = json.RawMessage(s.Profile.ParamsJSON)
	}
	dto.Teams = teamsDTO(s.TeamByPlayer)
	dto.Result = toResultDTO(s.Result)
	return dto
}

// playerIDs extrai os playerIDs dos participantes, na ordem em que entraram.
func playerIDs(participants []domain.Participant) []string {
	out := make([]string, len(participants))
	for i, p := range participants {
		out[i] = p.PlayerID
	}
	return out
}

// participantsDTO serializa os participantes (spec 025); nil (campo omitido)
// quando a sessão não tem nenhum registrado.
func participantsDTO(participants []domain.Participant) []participantDTO {
	if len(participants) == 0 {
		return nil
	}
	out := make([]participantDTO, len(participants))
	for i, p := range participants {
		out[i] = participantDTO{
			PlayerID:    p.PlayerID,
			Role:        p.Role,
			JoinedAt:    p.JoinedAt,
			LastEvent:   p.LastEvent,
			LastOutcome: p.LastOutcome,
		}
	}
	return out
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
		out = append(out, roleSlotDTO{Role: role, Capacity: slot.Capacity, Occupied: len(slot.Occupants), Occupants: slot.Occupants})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Role < out[j].Role })
	return out
}
