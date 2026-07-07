// Package registryclient é o adapter de saída que lê o perfil de multiplayer de
// um jogo no game-registry (ADR-0023) para o arquétipo host_world_dropin.
// Satisfaz domain.ProfileProvider. A internal key (ADR-0018) é injetada pelo
// transport do http.Client (auth.InternalKeyTransport), montado no main — como
// no adapter equivalente do matchmaking.
package registryclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/domain"
)

var _ domain.ProfileProvider = (*Client)(nil)

// archetypeHostWorldDropIn é o arquétipo que o session sabe abrir como mundo
// (ADR-0023). Vocabulário do game-registry.
const archetypeHostWorldDropIn = "host_world_dropin"

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

// profileResponse espelha o subconjunto do perfil que o session precisa para
// abrir um mundo. Tags conforme o game-registry (hostWorldDropInParamsDTO).
type profileResponse struct {
	Archetype string `json:"archetype"`
	Params    struct {
		EnabledRoles []string `json:"enabledRoles"`
		CoopSlots    int      `json:"coopSlots"`
		InvaderSlots int      `json:"invaderSlots"`
	} `json:"params"`
}

// WorldProfile lê o perfil do jogo e devolve os papéis/capacidade do mundo.
// Qualquer falha (jogo/perfil ausente, arquétipo != host_world_dropin, rede)
// vira ErrProfileUnavailable — o service a traduz para ErrUnsupportedArchetype
// no contexto do OpenWorld.
func (c *Client) WorldProfile(ctx context.Context, gameID string) (domain.WorldProfile, error) {
	u := c.baseURL + "/games/" + url.PathEscape(gameID) + "/multiplayer-profile"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return domain.WorldProfile{}, fmt.Errorf("registryclient: build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return domain.WorldProfile{}, fmt.Errorf("registryclient: get profile: %v: %w", err, domain.ErrProfileUnavailable)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return domain.WorldProfile{}, fmt.Errorf("registryclient: get profile: status %d: %w", resp.StatusCode, domain.ErrProfileUnavailable)
	}

	var p profileResponse
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return domain.WorldProfile{}, fmt.Errorf("registryclient: decode profile: %v: %w", err, domain.ErrProfileUnavailable)
	}
	if p.Archetype != archetypeHostWorldDropIn {
		return domain.WorldProfile{}, fmt.Errorf("registryclient: archetype %q não é host_world_dropin: %w", p.Archetype, domain.ErrProfileUnavailable)
	}

	return domain.WorldProfile{
		Archetype:    p.Archetype,
		EnabledRoles: p.Params.EnabledRoles,
		CoopSlots:    p.Params.CoopSlots,
		InvaderSlots: p.Params.InvaderSlots,
	}, nil
}
