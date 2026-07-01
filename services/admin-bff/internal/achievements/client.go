// Package achievements é o adapter de saída que lê/escreve a API admin do
// achievements-service (por jogo, ADR-0017) e satisfaz domain.AchievementsReader.
// Erros de validação do upstream (4xx) viram domain.UpstreamError;
// indisponibilidade vira domain.ErrAchievementsUnavailable.
package achievements

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/vitormartins1/go-game-backend/services/admin-bff/internal/domain"
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

func (c *Client) gamePath(gameID, suffix string) string {
	return c.baseURL + "/admin/games/" + url.PathEscape(gameID) + suffix
}

// catalogResponse é o shape de GET /admin/games/{g}/achievements.
type catalogResponse struct {
	Achievements []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		EventType   string `json:"eventType"`
		Target      int    `json:"target"`
	} `json:"achievements"`
}

// CatalogForGame lê o catálogo de conquistas do jogo.
func (c *Client) CatalogForGame(ctx context.Context, gameID string) ([]domain.Achievement, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.gamePath(gameID, "/achievements"), nil)
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var body catalogResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("achievements: decode: %w", err)
	}
	out := make([]domain.Achievement, 0, len(body.Achievements))
	for _, a := range body.Achievements {
		out = append(out, domain.Achievement{ID: a.ID, Name: a.Name, Description: a.Description, EventType: a.EventType, Target: a.Target})
	}
	return out, nil
}

// CreateAchievement cadastra uma conquista no catálogo do jogo. Validação (400/
// 409) vira UpstreamError.
func (c *Client) CreateAchievement(ctx context.Context, gameID string, a domain.Achievement) error {
	body, _ := json.Marshal(map[string]any{
		"id": a.ID, "name": a.Name, "description": a.Description,
		"eventType": a.EventType, "target": a.Target,
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.gamePath(gameID, "/achievements"), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	return nil
}

// UpdateAchievement edita uma conquista do jogo (PUT). 400/404 -> UpstreamError.
func (c *Client) UpdateAchievement(ctx context.Context, gameID, id string, a domain.Achievement) error {
	body, _ := json.Marshal(map[string]any{
		"name": a.Name, "description": a.Description, "eventType": a.EventType, "target": a.Target,
	})
	u := c.gamePath(gameID, "/achievements/"+url.PathEscape(id))
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	return nil
}

// DeleteAchievement remove uma conquista do jogo (DELETE). 404 -> UpstreamError.
func (c *Client) DeleteAchievement(ctx context.Context, gameID, id string) error {
	u := c.gamePath(gameID, "/achievements/"+url.PathEscape(id))
	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	return nil
}

// playerAchievementsResponse é o shape de .../players/{p}/achievements.
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

// PlayerAchievements lê os achievements do jogador no jogo.
func (c *Client) PlayerAchievements(ctx context.Context, gameID, playerID string) ([]domain.PlayerAchievement, error) {
	u := c.gamePath(gameID, "/players/"+url.PathEscape(playerID)+"/achievements")
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

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

// do executa a request e classifica a resposta (2xx ok; 4xx -> UpstreamError;
// conn/5xx -> ErrAchievementsUnavailable). Devolve a resposta só no caminho ok.
func (c *Client) do(req *http.Request) (*http.Response, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrAchievementsUnavailable, err)
	}
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return resp, nil
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		defer func() { _ = resp.Body.Close() }()
		return nil, &domain.UpstreamError{Status: resp.StatusCode, Code: errorCode(resp)}
	default:
		_ = resp.Body.Close()
		return nil, fmt.Errorf("%w: status %d", domain.ErrAchievementsUnavailable, resp.StatusCode)
	}
}

func errorCode(resp *http.Response) string {
	var body struct {
		Error string `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return body.Error
}
