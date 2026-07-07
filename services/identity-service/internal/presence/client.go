// Package presence é o adapter de saída que marca a presença do jogador num
// jogo, chamando a API interna do game-registry (ADR-0019). Satisfaz
// domain.PresenceNotifier. A internal key (ADR-0018) é injetada pelo transport
// do http.Client (auth.InternalKeyTransport), montado no main.
package presence

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/identity-service/internal/domain"
)

var _ domain.PresenceNotifier = (*Client)(nil)

// Client chama o game-registry em baseURL (ex.: http://game-registry:8080).
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

// MarkPresence faz PUT /games/{gameId}/players/{playerID} no game-registry.
func (c *Client) MarkPresence(ctx context.Context, gameID, playerID, handle string) error {
	u := c.baseURL + "/games/" + url.PathEscape(gameID) + "/players/" + url.PathEscape(playerID)
	body, _ := json.Marshal(map[string]string{"handle": handle})
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("presence: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("presence: mark: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("presence: mark: status %d", resp.StatusCode)
	}
	return nil
}
