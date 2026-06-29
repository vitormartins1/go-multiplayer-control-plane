// Package achievements é o adapter de saída que lê a API admin do
// achievements-service e satisfaz domain.AchievementsReader (ADR-0002/0016).
package achievements

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/c130n/game-backend/services/admin-bff/internal/domain"
)

var _ domain.AchievementsReader = (*Client)(nil)

// Client lê o achievements-service em baseURL (ex.: http://achievements-service:8080).
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

// playerAchievementsResponse é o shape do /admin/players/{id}/achievements.
type playerAchievementsResponse struct {
	Achievements []struct {
		ID          string     `json:"id"`
		Name        string     `json:"name"`
		Description string     `json:"description"`
		Progress    int        `json:"progress"`
		Target      int        `json:"target"`
		Unlocked    bool       `json:"unlocked"`
		UnlockedAt  *time.Time `json:"unlocked_at"`
	} `json:"achievements"`
}

// ForPlayer lê os achievements do jogador. Falha de conexão/HTTP vira
// ErrAchievementsUnavailable.
func (c *Client) ForPlayer(ctx context.Context, playerID string) ([]domain.PlayerAchievement, error) {
	u := c.baseURL + "/admin/players/" + url.PathEscape(playerID) + "/achievements"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("achievements: build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrAchievementsUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", domain.ErrAchievementsUnavailable, resp.StatusCode)
	}

	var body playerAchievementsResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("achievements: decode: %w", err)
	}

	out := make([]domain.PlayerAchievement, 0, len(body.Achievements))
	for _, a := range body.Achievements {
		pa := domain.PlayerAchievement{
			ID: a.ID, Name: a.Name, Description: a.Description,
			Target: a.Target, Progress: a.Progress, Unlocked: a.Unlocked,
		}
		if a.UnlockedAt != nil {
			pa.UnlockedAt = *a.UnlockedAt
		}
		out = append(out, pa)
	}
	return out, nil
}
