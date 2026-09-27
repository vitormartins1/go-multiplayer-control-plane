// Package playerdata é o adapter de saída que lê a API admin do
// player-data-service (dado de gameplay por jogo+jogador, ADR-0017) e satisfaz
// domain.PlayerDataReader. Indisponibilidade vira domain.ErrPlayerDataUnavailable;
// erros de validação do upstream (4xx) viram domain.UpstreamError.
package playerdata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/admin-bff/internal/domain"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/admin-bff/internal/upstream"
)

var _ domain.PlayerDataReader = (*Client)(nil)

// Client lê o player-data-service em baseURL (ex.: http://player-data-service:8080).
type Client struct {
	baseURL string
	http    *http.Client
}

// New cria o client. O httpClient permite injetar timeout/transport (otelhttp +
// internal key).
func New(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = upstream.DefaultClient()
	}
	return &Client{baseURL: baseURL, http: httpClient}
}

// dataResponse é o shape de GET /admin/games/{g}/players/{p}/data.
type dataResponse struct {
	Data []struct {
		Namespace string          `json:"namespace"`
		Key       string          `json:"key"`
		Value     json.RawMessage `json:"value"`
	} `json:"data"`
}

// ForPlayer lê todo o dado de um jogador num jogo.
func (c *Client) ForPlayer(ctx context.Context, gameID, playerID string) ([]domain.PlayerDataEntry, error) {
	u := c.baseURL + "/admin/games/" + url.PathEscape(gameID) + "/players/" + url.PathEscape(playerID) + "/data"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	resp, err := upstream.Do(c.http, req, domain.ErrPlayerDataUnavailable)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var body dataResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("playerdata: decode: %w", err)
	}
	out := make([]domain.PlayerDataEntry, 0, len(body.Data))
	for _, e := range body.Data {
		out = append(out, domain.PlayerDataEntry{Namespace: e.Namespace, Key: e.Key, Value: e.Value})
	}
	return out, nil
}
