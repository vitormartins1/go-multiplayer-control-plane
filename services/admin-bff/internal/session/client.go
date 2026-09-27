// Package session é o adapter de saída que lê a API admin do session-service
// (spec 007-sessions-panel) e satisfaz domain.SessionsReader. Proxy fino: o
// enriquecimento de handles é do service.
package session

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/admin-bff/internal/domain"
)

var _ domain.SessionsReader = (*Client)(nil)

// Client lê o session-service em baseURL (ex.: http://session-service:8080).
type Client struct {
	baseURL string
	http    *http.Client
}

// New cria o client. O httpClient permite injetar timeout/transport (otelhttp +
// internal key).
func New(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}
	return &Client{baseURL: baseURL, http: httpClient}
}

// sessionsResponse é o shape de GET /admin/games/{g}/sessions do session-service.
type sessionsResponse struct {
	Active []sessionDTO `json:"active"`
	Closed []sessionDTO `json:"closed"`
}

type sessionDTO struct {
	SessionID     string     `json:"sessionId"`
	MatchID       string     `json:"matchId"`
	GameID        string     `json:"gameId"`
	ModeID        string     `json:"modeId"` // modo do jogo (ADR-0024, aditivo)
	State         string     `json:"state"`
	Players       []string   `json:"players"`
	ServerAddress string     `json:"serverAddress"`
	OpenedAt      time.Time  `json:"openedAt"`
	ClosedAt      *time.Time `json:"closedAt"`
	CloseReason   string     `json:"closeReason"`
	Origin        string     `json:"origin"`
	HostPlayerID  string     `json:"hostPlayerId"`
	RoleSlots     []struct {
		Role      string   `json:"role"`
		Capacity  int      `json:"capacity"`
		Occupied  int      `json:"occupied"`
		Occupants []string `json:"occupants"`
	} `json:"roleSlots"`

	// Detalhe rico (session spec 018) — passthrough 1:1; os tipos do domain já
	// espelham o shape do session, então decodifica direto neles.
	Archetype string `json:"archetype"`
	Topology  string `json:"topology"` // session/031
	// admin-ui/091: conexão do host p2p e último heartbeat (session/032).
	HostConnection  string               `json:"hostConnection"`
	LastHeartbeatAt *time.Time           `json:"lastHeartbeatAt"`
	ProfileHash     string               `json:"profileHash"`
	ModeParams      json.RawMessage      `json:"modeParams"`
	Teams           []domain.SessionTeam `json:"teams"`
	Result          *domain.MatchResult  `json:"result"`

	// Participants é a base do histórico persistido (spec 025).
	Participants []participantDTO `json:"participants"`
}

// participantDTO espelha domain.SessionParticipant (spec 025) — sem Handle
// (o session-service não conhece identity; o service do admin-bff enriquece).
// LastEvent/LastOutcome (spec 027 do session-service): último evento de
// participação conhecido — só existem em host_world_dropin.
type participantDTO struct {
	PlayerID    string    `json:"playerId"`
	Role        string    `json:"role"`
	JoinedAt    time.Time `json:"joinedAt"`
	LastEvent   string    `json:"lastEvent"`
	LastOutcome string    `json:"lastOutcome"`
}

// SessionsByGame lê as sessões do jogo, repassando o filtro state.
func (c *Client) SessionsByGame(ctx context.Context, gameID, state string) (domain.GameSessions, error) {
	return c.getSessions(ctx, c.baseURL+"/admin/games/"+url.PathEscape(gameID)+"/sessions", state)
}

// AllSessions lê as sessões de todos os jogos (board global do console, 008).
func (c *Client) AllSessions(ctx context.Context, state string) (domain.GameSessions, error) {
	return c.getSessions(ctx, c.baseURL+"/admin/sessions", state)
}

// getSessions faz o GET (com filtro state) e mapeia — compartilhado entre a
// listagem por-jogo e a global.
func (c *Client) getSessions(ctx context.Context, u, state string) (domain.GameSessions, error) {
	if state != "" {
		u += "?state=" + url.QueryEscape(state)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return domain.GameSessions{}, fmt.Errorf("session: build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return domain.GameSessions{}, fmt.Errorf("%w: %v", domain.ErrSessionsUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return domain.GameSessions{}, fmt.Errorf("%w: status %d", domain.ErrSessionsUnavailable, resp.StatusCode)
	}

	var body sessionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return domain.GameSessions{}, fmt.Errorf("session: decode: %w", err)
	}
	return domain.GameSessions{Active: toGameSessions(body.Active), Closed: toGameSessions(body.Closed)}, nil
}

// CloseSession encerra a sessão ativa (force-close, 008). Propaga o status do
// session: 404 -> UpstreamError{Status:404}; conexão fora -> ErrSessionsUnavailable.
func (c *Client) CloseSession(ctx context.Context, matchID string) (domain.GameSession, error) {
	u := c.baseURL + "/admin/sessions/" + url.PathEscape(matchID) + "/close"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, nil)
	if err != nil {
		return domain.GameSession{}, fmt.Errorf("session: build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return domain.GameSession{}, fmt.Errorf("%w: %v", domain.ErrSessionsUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return domain.GameSession{}, &domain.UpstreamError{Status: resp.StatusCode}
	}

	var d sessionDTO
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return domain.GameSession{}, fmt.Errorf("session: decode: %w", err)
	}
	return toGameSessions([]sessionDTO{d})[0], nil
}

// closedPageDTO é o shape paginado de GET .../sessions?state=closed e de
// GET /admin/players/{p}/sessions do session-service (spec 025).
type closedPageDTO struct {
	Closed     []sessionDTO `json:"closed"`
	NextBefore *time.Time   `json:"nextBefore"`
}

// ClosedSessionsByGame lê as sessões fechadas persistidas do jogo, paginadas.
func (c *Client) ClosedSessionsByGame(ctx context.Context, gameID string, limit int, before time.Time) (domain.ClosedSessionsPage, error) {
	u := c.baseURL + "/admin/games/" + url.PathEscape(gameID) + "/sessions?state=closed" + paginationQuery(limit, before)
	return c.getClosedPage(ctx, u)
}

// PlayerSessions lê as sessões fechadas do jogo em que o player participou
// (spec 025, aba History do player), paginadas.
func (c *Client) PlayerSessions(ctx context.Context, playerID, gameID string, limit int, before time.Time) (domain.ClosedSessionsPage, error) {
	u := c.baseURL + "/admin/players/" + url.PathEscape(playerID) + "/sessions?gameId=" + url.QueryEscape(gameID) + paginationQuery(limit, before)
	return c.getClosedPage(ctx, u)
}

// paginationQuery monta o sufixo de query string de limit/before — before
// zero (sem cursor) e limit <=0 (default do session-service) ficam de fora.
func paginationQuery(limit int, before time.Time) string {
	q := ""
	if limit > 0 {
		q += "&limit=" + strconv.Itoa(limit)
	}
	if !before.IsZero() {
		q += "&before=" + url.QueryEscape(before.Format(time.RFC3339))
	}
	return q
}

// getClosedPage faz o GET e mapeia a resposta paginada. 503 do session-service
// (Postgres do histórico fora, plan D6) -> ErrHistoryUnavailable, distinto de
// ErrSessionsUnavailable (session-service inteiro inacessível).
func (c *Client) getClosedPage(ctx context.Context, u string) (domain.ClosedSessionsPage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return domain.ClosedSessionsPage{}, fmt.Errorf("session: build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return domain.ClosedSessionsPage{}, fmt.Errorf("%w: %v", domain.ErrSessionsUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusServiceUnavailable:
		return domain.ClosedSessionsPage{}, domain.ErrHistoryUnavailable
	default:
		return domain.ClosedSessionsPage{}, &domain.UpstreamError{Status: resp.StatusCode}
	}

	var body closedPageDTO
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return domain.ClosedSessionsPage{}, fmt.Errorf("session: decode: %w", err)
	}
	return domain.ClosedSessionsPage{Closed: toGameSessions(body.Closed), NextBefore: body.NextBefore}, nil
}

// fleetStatusDTO é o shape de GET /admin/fleet-status/{fleetName} do
// session-service (spec 023) — mesmos campos do domain.FleetStatus.
type fleetStatusDTO struct {
	Cluster     string `json:"cluster"`
	FleetExists bool   `json:"fleetExists"`
	Desired     int    `json:"desired"`
	Current     int    `json:"current"`
	Ready       int    `json:"ready"`
	Allocated   int    `json:"allocated"`
}

// FleetStatus lê o status real da Fleet pelo nome (spec 023).
func (c *Client) FleetStatus(ctx context.Context, fleetName string) (domain.FleetStatus, error) {
	u := c.baseURL + "/admin/fleet-status/" + url.PathEscape(fleetName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return domain.FleetStatus{}, fmt.Errorf("session: build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return domain.FleetStatus{}, fmt.Errorf("%w: %v", domain.ErrSessionsUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return domain.FleetStatus{}, fmt.Errorf("%w: status %d", domain.ErrSessionsUnavailable, resp.StatusCode)
	}

	var d fleetStatusDTO
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return domain.FleetStatus{}, fmt.Errorf("session: decode: %w", err)
	}
	return domain.FleetStatus{
		Cluster:     d.Cluster,
		FleetExists: d.FleetExists,
		Desired:     d.Desired,
		Current:     d.Current,
		Ready:       d.Ready,
		Allocated:   d.Allocated,
	}, nil
}

func toGameSessions(dtos []sessionDTO) []domain.GameSession {
	out := make([]domain.GameSession, 0, len(dtos))
	for _, d := range dtos {
		players := make([]domain.SessionPlayer, 0, len(d.Players))
		for _, id := range d.Players {
			players = append(players, domain.SessionPlayer{ID: id}) // handle: service enriquece
		}
		var slots []domain.RoleSlot
		for _, s := range d.RoleSlots {
			slots = append(slots, domain.RoleSlot{Role: s.Role, Capacity: s.Capacity, Occupied: s.Occupied, Occupants: s.Occupants})
		}
		var participants []domain.SessionParticipant
		for _, p := range d.Participants {
			participants = append(participants, domain.SessionParticipant{
				PlayerID: p.PlayerID, Role: p.Role, JoinedAt: p.JoinedAt, // handle: service enriquece
				LastEvent: p.LastEvent, LastOutcome: p.LastOutcome,
			})
		}
		out = append(out, domain.GameSession{
			SessionID:       d.SessionID,
			MatchID:         d.MatchID,
			GameID:          d.GameID,
			ModeID:          d.ModeID,
			State:           d.State,
			Players:         players,
			ServerAddress:   d.ServerAddress,
			OpenedAt:        d.OpenedAt,
			ClosedAt:        d.ClosedAt,
			CloseReason:     d.CloseReason,
			Origin:          d.Origin,
			HostPlayerID:    d.HostPlayerID,
			RoleSlots:       slots,
			Archetype:       d.Archetype,
			Topology:        d.Topology,
			HostConnection:  d.HostConnection,
			LastHeartbeatAt: d.LastHeartbeatAt,
			ProfileHash:     d.ProfileHash,
			ModeParams:      d.ModeParams,
			Teams:           d.Teams,
			Result:          d.Result,
			Participants:    participants,
		})
	}
	return out
}
