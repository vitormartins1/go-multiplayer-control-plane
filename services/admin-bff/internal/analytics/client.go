// Package analytics é o adapter de saída que lê a read API do analytics-service
// (ADR-0021) e satisfaz domain.AnalyticsReader. Proxy fino: o enriquecimento com
// o nome do jogo é do service.
package analytics

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

var _ domain.AnalyticsReader = (*Client)(nil)

// Client lê o analytics-service em baseURL (ex.: http://analytics-service:8080).
type Client struct {
	baseURL string
	http    *http.Client
}

// New cria o client (httpClient injeta timeout/transport + internal key).
func New(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}
	return &Client{baseURL: baseURL, http: httpClient}
}

// Overview lê GET /admin/analytics/overview. Indisponibilidade -> ErrAnalyticsUnavailable.
func (c *Client) Overview(ctx context.Context) (domain.AnalyticsOverview, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/admin/analytics/overview", nil)
	if err != nil {
		return domain.AnalyticsOverview{}, fmt.Errorf("analytics: build request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return domain.AnalyticsOverview{}, fmt.Errorf("%w: %v", domain.ErrAnalyticsUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return domain.AnalyticsOverview{}, fmt.Errorf("%w: status %d", domain.ErrAnalyticsUnavailable, resp.StatusCode)
	}

	var ov domain.AnalyticsOverview
	if err := json.NewDecoder(resp.Body).Decode(&ov); err != nil {
		return domain.AnalyticsOverview{}, fmt.Errorf("analytics: decode: %w", err)
	}
	return ov, nil
}

// GameAnalytics lê GET /admin/games/{id}/analytics (detalhe por jogo, com
// roleParticipations). Indisponibilidade -> ErrAnalyticsUnavailable.
func (c *Client) GameAnalytics(ctx context.Context, gameID string, days int) (domain.GameAnalyticsDetail, error) {
	u := c.baseURL + "/admin/games/" + url.PathEscape(gameID) + "/analytics"
	if days > 0 {
		u += "?days=" + strconv.Itoa(days)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return domain.GameAnalyticsDetail{}, fmt.Errorf("analytics: build request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return domain.GameAnalyticsDetail{}, fmt.Errorf("%w: %v", domain.ErrAnalyticsUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return domain.GameAnalyticsDetail{}, fmt.Errorf("%w: status %d", domain.ErrAnalyticsUnavailable, resp.StatusCode)
	}

	var d domain.GameAnalyticsDetail
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return domain.GameAnalyticsDetail{}, fmt.Errorf("analytics: decode: %w", err)
	}
	return d, nil
}
