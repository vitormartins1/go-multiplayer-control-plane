// Package identity é o adapter de saída que lê a API admin do identity-service
// e satisfaz domain.PlayerDirectory (ADR-0002/0016). Proxy fino: repassa a
// paginação (a política é do identity).
package identity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/vitormartins1/go-game-backend/services/admin-bff/internal/domain"
)

var _ domain.PlayerDirectory = (*Client)(nil)

// Client lê o identity-service em baseURL (ex.: http://identity-service:8080).
type Client struct {
	baseURL string
	http    *http.Client
}

// New cria o client. O httpClient permite injetar timeout/transport (otelhttp).
func New(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}
	return &Client{baseURL: baseURL, http: httpClient}
}

// playersResponse é o shape do GET /admin/players do identity.
type playersResponse struct {
	Total   int `json:"total"`
	Players []struct {
		ID        string    `json:"id"`
		Handle    string    `json:"handle"`
		CreatedAt time.Time `json:"createdAt"`
	} `json:"players"`
}

// ListPlayers chama o /admin/players do identity, repassando limit/offset.
// Falha de conexão/HTTP vira ErrIdentityUnavailable.
func (c *Client) ListPlayers(ctx context.Context, limit, offset int) (domain.PlayerPage, error) {
	q := url.Values{"limit": {strconv.Itoa(limit)}, "offset": {strconv.Itoa(offset)}}
	u := c.baseURL + "/admin/players?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return domain.PlayerPage{}, fmt.Errorf("identity: build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return domain.PlayerPage{}, fmt.Errorf("%w: %v", domain.ErrIdentityUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return domain.PlayerPage{}, fmt.Errorf("%w: status %d", domain.ErrIdentityUnavailable, resp.StatusCode)
	}

	var body playersResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return domain.PlayerPage{}, fmt.Errorf("identity: decode: %w", err)
	}

	page := domain.PlayerPage{Total: body.Total, Players: make([]domain.Player, len(body.Players))}
	for i, p := range body.Players {
		page.Players[i] = domain.Player{ID: p.ID, Handle: p.Handle, CreatedAt: p.CreatedAt}
	}
	return page, nil
}
