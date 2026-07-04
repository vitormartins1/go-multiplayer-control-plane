// Package gameregistry é o adapter de saída para o game-registry (gestão de
// jogos) e satisfaz domain.GameRegistry (ADR-0002/0017). Erros de validação do
// upstream (4xx) viram domain.UpstreamError (status repassado); indisponibilidade
// vira domain.ErrGameRegistryUnavailable.
package gameregistry

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

var _ domain.GameRegistry = (*Client)(nil)

// Client lê/escreve no game-registry em baseURL (ex.: http://game-registry:8080).
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

func (c *Client) ListGames(ctx context.Context) ([]domain.Game, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/games", nil)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrGameRegistryUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if err := classify(resp); err != nil {
		return nil, err
	}
	var games []domain.Game
	if err := json.NewDecoder(resp.Body).Decode(&games); err != nil {
		return nil, fmt.Errorf("gameregistry: decode: %w", err)
	}
	return games, nil
}

func (c *Client) CreateGame(ctx context.Context, slug, name string) (*domain.Game, error) {
	body, _ := json.Marshal(map[string]string{"slug": slug, "name": name})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/games", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrGameRegistryUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if err := classify(resp); err != nil {
		return nil, err
	}
	var g domain.Game
	if err := json.NewDecoder(resp.Body).Decode(&g); err != nil {
		return nil, fmt.Errorf("gameregistry: decode: %w", err)
	}
	return &g, nil
}

// RenameGame altera o nome do jogo (PUT /games/{id}).
func (c *Client) RenameGame(ctx context.Context, id, name string) (*domain.Game, error) {
	body, _ := json.Marshal(map[string]string{"name": name})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+"/games/"+url.PathEscape(id), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrGameRegistryUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if err := classify(resp); err != nil {
		return nil, err
	}
	var g domain.Game
	if err := json.NewDecoder(resp.Body).Decode(&g); err != nil {
		return nil, fmt.Errorf("gameregistry: decode: %w", err)
	}
	return &g, nil
}

// PlayersOf lê a player base do jogo (presença, ADR-0019).
func (c *Client) PlayersOf(ctx context.Context, gameID string) ([]domain.GamePlayer, error) {
	u := c.baseURL + "/games/" + url.PathEscape(gameID) + "/players"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrGameRegistryUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if err := classify(resp); err != nil {
		return nil, err
	}
	var players []domain.GamePlayer
	if err := json.NewDecoder(resp.Body).Decode(&players); err != nil {
		return nil, fmt.Errorf("gameregistry: decode: %w", err)
	}
	return players, nil
}

// GamesOfPlayer lê os jogos em que o player está presente (consulta reversa).
func (c *Client) GamesOfPlayer(ctx context.Context, playerID string) ([]domain.Game, error) {
	u := c.baseURL + "/players/" + url.PathEscape(playerID) + "/games"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrGameRegistryUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if err := classify(resp); err != nil {
		return nil, err
	}
	var games []domain.Game
	if err := json.NewDecoder(resp.Body).Decode(&games); err != nil {
		return nil, fmt.Errorf("gameregistry: decode: %w", err)
	}
	return games, nil
}

// classify mapeia a resposta do upstream: 2xx -> nil; 4xx -> UpstreamError (status
// + code do corpo); conn/5xx (já tratado no Do) ou 5xx -> ErrGameRegistryUnavailable.
func classify(resp *http.Response) error {
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		return &domain.UpstreamError{Status: resp.StatusCode, Code: errorCode(resp)}
	default:
		return fmt.Errorf("%w: status %d", domain.ErrGameRegistryUnavailable, resp.StatusCode)
	}
}

// errorCode extrai o campo "error" do corpo, se houver.
func errorCode(resp *http.Response) string {
	var body struct {
		Error string `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return body.Error
}
