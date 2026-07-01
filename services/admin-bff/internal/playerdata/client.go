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
	"time"

	"github.com/vitormartins1/go-game-backend/services/admin-bff/internal/domain"
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
		httpClient = &http.Client{Timeout: 5 * time.Second}
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
	resp, err := c.do(req)
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

// do executa a request e classifica a resposta (2xx ok; 4xx -> UpstreamError;
// conn/5xx -> ErrPlayerDataUnavailable). Devolve a resposta só no caminho ok.
func (c *Client) do(req *http.Request) (*http.Response, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrPlayerDataUnavailable, err)
	}
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return resp, nil
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		defer func() { _ = resp.Body.Close() }()
		return nil, &domain.UpstreamError{Status: resp.StatusCode, Code: errorCode(resp)}
	default:
		_ = resp.Body.Close()
		return nil, fmt.Errorf("%w: status %d", domain.ErrPlayerDataUnavailable, resp.StatusCode)
	}
}

func errorCode(resp *http.Response) string {
	var body struct {
		Error string `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return body.Error
}
