// Package registryclient é o adapter de saída que lê o perfil de multiplayer de
// um jogo no game-registry (ADR-0023) e deriva o tamanho do match. Satisfaz
// domain.ProfileProvider. A internal key (ADR-0018) é injetada pelo transport do
// http.Client (auth.InternalKeyTransport), montado no main — como no adapter de
// presença do identity.
package registryclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/vitormartins1/go-game-backend/services/matchmaking-service/internal/domain"
)

var _ domain.ProfileProvider = (*Client)(nil)

// archetypeQueueArena é o único arquétipo cujo tamanho o matchmaking sabe
// derivar nesta fatia (ADR-0023). Vocabulário do game-registry.
const archetypeQueueArena = "queue_arena"

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

// profileResponse espelha o subconjunto do perfil que importa para o tamanho.
type profileResponse struct {
	Archetype string `json:"archetype"`
	Params    struct {
		Structure   string `json:"structure"`
		PlayerCount int    `json:"playerCount"`
		TeamCount   int    `json:"teamCount"`
		TeamSize    int    `json:"teamSize"`
	} `json:"params"`
}

// MatchSize lê o perfil do jogo e deriva o tamanho do match. Qualquer falha
// (jogo/perfil ausente, arquétipo != queue_arena, tamanho inválido, rede) vira
// ErrProfileUnavailable — o ticket não deve entrar na fila.
func (c *Client) MatchSize(ctx context.Context, gameID string) (int, error) {
	u := c.baseURL + "/games/" + url.PathEscape(gameID) + "/multiplayer-profile"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, fmt.Errorf("registryclient: build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("registryclient: get profile: %v: %w", err, domain.ErrProfileUnavailable)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("registryclient: get profile: status %d: %w", resp.StatusCode, domain.ErrProfileUnavailable)
	}

	var p profileResponse
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return 0, fmt.Errorf("registryclient: decode profile: %v: %w", err, domain.ErrProfileUnavailable)
	}
	if p.Archetype != archetypeQueueArena {
		return 0, fmt.Errorf("registryclient: archetype %q não suportado: %w", p.Archetype, domain.ErrProfileUnavailable)
	}

	size := 0
	switch p.Params.Structure {
	case "ffa":
		size = p.Params.PlayerCount
	case "teams":
		size = p.Params.TeamCount * p.Params.TeamSize
	}
	if size <= 0 {
		return 0, fmt.Errorf("registryclient: tamanho inválido (%d) para structure %q: %w",
			size, p.Params.Structure, domain.ErrProfileUnavailable)
	}
	return size, nil
}
