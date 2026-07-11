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
	"io"
	"net/http"
	"net/url"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/admin-bff/internal/domain"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/admin-bff/internal/upstream"
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
		httpClient = upstream.DefaultClient()
	}
	return &Client{baseURL: baseURL, http: httpClient}
}

func (c *Client) ListGames(ctx context.Context) ([]domain.Game, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/games", nil)
	resp, err := upstream.Do(c.http, req, domain.ErrGameRegistryUnavailable)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
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
	resp, err := upstream.Do(c.http, req, domain.ErrGameRegistryUnavailable)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
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
	resp, err := upstream.Do(c.http, req, domain.ErrGameRegistryUnavailable)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var g domain.Game
	if err := json.NewDecoder(resp.Body).Decode(&g); err != nil {
		return nil, fmt.Errorf("gameregistry: decode: %w", err)
	}
	return &g, nil
}

// SetFleet cadastra/troca a Fleet Agones do jogo (PUT /games/{id}/fleet,
// spec 012 do game-registry — endpoint próprio, separado do PUT genérico).
func (c *Client) SetFleet(ctx context.Context, id, fleet string) (*domain.Game, error) {
	body, _ := json.Marshal(map[string]string{"fleet": fleet})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+"/games/"+url.PathEscape(id)+"/fleet", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := upstream.Do(c.http, req, domain.ErrGameRegistryUnavailable)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var g domain.Game
	if err := json.NewDecoder(resp.Body).Decode(&g); err != nil {
		return nil, fmt.Errorf("gameregistry: decode: %w", err)
	}
	return &g, nil
}

// SetDescription atualiza a descrição do jogo (PUT /games/{id}/description,
// spec 013 do game-registry — endpoint próprio, mesmo molde do fleet).
func (c *Client) SetDescription(ctx context.Context, id, description string) (*domain.Game, error) {
	body, _ := json.Marshal(map[string]string{"description": description})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+"/games/"+url.PathEscape(id)+"/description", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := upstream.Do(c.http, req, domain.ErrGameRegistryUnavailable)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var g domain.Game
	if err := json.NewDecoder(resp.Body).Decode(&g); err != nil {
		return nil, fmt.Errorf("gameregistry: decode: %w", err)
	}
	return &g, nil
}

// DeleteGame remove o jogo (DELETE /games/{id} — soft delete no registry,
// ADR-0025). 204 esperado; 404 (inexistente/já deletado) vira UpstreamError.
func (c *Client) DeleteGame(ctx context.Context, id string) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/games/"+url.PathEscape(id), nil)
	resp, err := upstream.Do(c.http, req, domain.ErrGameRegistryUnavailable)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	return nil
}

// GetMultiplayerProfile lê o perfil de multiplayer do jogo (ADR-0023). Devolve o
// corpo cru — o BFF não interpreta a forma (que cresce por arquétipo).
func (c *Client) GetMultiplayerProfile(ctx context.Context, gameID string) (json.RawMessage, error) {
	u := c.baseURL + "/games/" + url.PathEscape(gameID) + "/multiplayer-profile"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	resp, err := upstream.Do(c.http, req, domain.ErrGameRegistryUnavailable)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gameregistry: read profile: %w", err)
	}
	return raw, nil
}

// SetMultiplayerProfile faz upsert do perfil (PUT, internal key). Repassa o corpo
// cru recebido do painel e devolve o perfil persistido cru.
func (c *Client) SetMultiplayerProfile(ctx context.Context, gameID string, profile json.RawMessage) (json.RawMessage, error) {
	u := c.baseURL + "/games/" + url.PathEscape(gameID) + "/multiplayer-profile"
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(profile))
	req.Header.Set("Content-Type", "application/json")
	resp, err := upstream.Do(c.http, req, domain.ErrGameRegistryUnavailable)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gameregistry: read profile: %w", err)
	}
	return raw, nil
}

// --- modos nomeados (ADR-0024, spec 017) — passthrough cru como o perfil ---

// doRaw executa a request e devolve o corpo cru (o BFF não interpreta a forma
// do modo, que cresce por arquétipo).
func (c *Client) doRaw(req *http.Request) (json.RawMessage, error) {
	resp, err := upstream.Do(c.http, req, domain.ErrGameRegistryUnavailable)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gameregistry: read: %w", err)
	}
	return raw, nil
}

func (c *Client) modeURL(gameID, modeID string) string {
	u := c.baseURL + "/games/" + url.PathEscape(gameID) + "/multiplayer-profiles"
	if modeID != "" {
		u += "/" + url.PathEscape(modeID)
	}
	return u
}

// ListMultiplayerModes lê a coleção de modos do jogo (cru).
func (c *Client) ListMultiplayerModes(ctx context.Context, gameID string) (json.RawMessage, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.modeURL(gameID, ""), nil)
	return c.doRaw(req)
}

// GetMultiplayerMode lê um modo (cru). UpstreamError(404) se jogo/modo ausente.
func (c *Client) GetMultiplayerMode(ctx context.Context, gameID, modeID string) (json.RawMessage, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.modeURL(gameID, modeID), nil)
	return c.doRaw(req)
}

// SetMultiplayerMode faz upsert de um modo repassando o corpo cru do painel.
func (c *Client) SetMultiplayerMode(ctx context.Context, gameID, modeID string, mode json.RawMessage) (json.RawMessage, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, c.modeURL(gameID, modeID), bytes.NewReader(mode))
	req.Header.Set("Content-Type", "application/json")
	return c.doRaw(req)
}

// DeleteMultiplayerMode remove um modo. UpstreamError(409) no default
// (default_mode_protected); 404 se jogo/modo ausente.
func (c *Client) DeleteMultiplayerMode(ctx context.Context, gameID, modeID string) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, c.modeURL(gameID, modeID), nil)
	resp, err := upstream.Do(c.http, req, domain.ErrGameRegistryUnavailable)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	return nil
}

// PlayersOf lê a player base do jogo (presença, ADR-0019).
func (c *Client) PlayersOf(ctx context.Context, gameID string) ([]domain.GamePlayer, error) {
	u := c.baseURL + "/games/" + url.PathEscape(gameID) + "/players"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	resp, err := upstream.Do(c.http, req, domain.ErrGameRegistryUnavailable)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
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
	resp, err := upstream.Do(c.http, req, domain.ErrGameRegistryUnavailable)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var games []domain.Game
	if err := json.NewDecoder(resp.Body).Decode(&games); err != nil {
		return nil, fmt.Errorf("gameregistry: decode: %w", err)
	}
	return games, nil
}
