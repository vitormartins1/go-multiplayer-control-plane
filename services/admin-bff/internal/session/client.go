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
		Role     string `json:"role"`
		Capacity int    `json:"capacity"`
		Occupied int    `json:"occupied"`
	} `json:"roleSlots"`
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

func toGameSessions(dtos []sessionDTO) []domain.GameSession {
	out := make([]domain.GameSession, 0, len(dtos))
	for _, d := range dtos {
		players := make([]domain.SessionPlayer, 0, len(d.Players))
		for _, id := range d.Players {
			players = append(players, domain.SessionPlayer{ID: id}) // handle: service enriquece
		}
		var slots []domain.RoleSlot
		for _, s := range d.RoleSlots {
			slots = append(slots, domain.RoleSlot{Role: s.Role, Capacity: s.Capacity, Occupied: s.Occupied})
		}
		out = append(out, domain.GameSession{
			SessionID:     d.SessionID,
			MatchID:       d.MatchID,
			GameID:        d.GameID,
			ModeID:        d.ModeID,
			State:         d.State,
			Players:       players,
			ServerAddress: d.ServerAddress,
			OpenedAt:      d.OpenedAt,
			ClosedAt:      d.ClosedAt,
			CloseReason:   d.CloseReason,
			Origin:        d.Origin,
			HostPlayerID:  d.HostPlayerID,
			RoleSlots:     slots,
		})
	}
	return out
}
