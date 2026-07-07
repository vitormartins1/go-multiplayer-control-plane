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

var (
	_ domain.ProfileProvider        = (*Client)(nil)
	_ domain.InvasionPolicyProvider = (*Client)(nil)
	_ domain.CoopPolicyProvider     = (*Client)(nil)
)

// roleCoop é o papel cujo habilitação (em params.enabledRoles) o quadro de sinais
// exige (spec 007). Vocabulário do game-registry.
const roleCoop = "coop"

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

// profileResponse espelha o subconjunto do perfil que importa para o tamanho
// (queue_arena) e para a política de invasão (host_world_dropin, spec 006).
type profileResponse struct {
	Archetype string `json:"archetype"`
	Params    struct {
		Structure   string `json:"structure"`
		PlayerCount int    `json:"playerCount"`
		TeamCount   int    `json:"teamCount"`
		TeamSize    int    `json:"teamSize"`
		// host_world_dropin (spec 006/007): flags que ligam/desligam cada filtro.
		// Ausentes em queue_arena (ficam false). enabledRoles lista os papéis
		// habilitados (coop/invader) — o co-op (spec 007) exige "coop".
		InvasionEnabled   bool     `json:"invasionEnabled"`
		LevelRangeEnabled bool     `json:"levelRangeEnabled"`
		WeaponTierEnabled bool     `json:"weaponTierEnabled"`
		EnabledRoles      []string `json:"enabledRoles"`
	} `json:"params"`
}

// getProfile faz o GET do perfil e decodifica. Erro de rede/status/parse vira
// ErrProfileUnavailable — compartilhado por Format (queue_arena) e InvasionPolicy.
func (c *Client) getProfile(ctx context.Context, gameID string) (profileResponse, error) {
	u := c.baseURL + "/games/" + url.PathEscape(gameID) + "/multiplayer-profile"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return profileResponse{}, fmt.Errorf("registryclient: build request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return profileResponse{}, fmt.Errorf("registryclient: get profile: %v: %w", err, domain.ErrProfileUnavailable)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return profileResponse{}, fmt.Errorf("registryclient: get profile: status %d: %w", resp.StatusCode, domain.ErrProfileUnavailable)
	}
	var p profileResponse
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return profileResponse{}, fmt.Errorf("registryclient: decode profile: %v: %w", err, domain.ErrProfileUnavailable)
	}
	return p, nil
}

// InvasionPolicy lê o perfil e monta a política de invasão (spec 006). Arquétipo
// != host_world_dropin devolve a policy com o Archetype preenchido (SupportsInvasion
// = false) — o service decide ErrInvasionNotSupported. Rede/404/parse →
// ErrProfileUnavailable.
func (c *Client) InvasionPolicy(ctx context.Context, gameID string) (domain.InvasionPolicy, error) {
	p, err := c.getProfile(ctx, gameID)
	if err != nil {
		return domain.InvasionPolicy{}, err
	}
	return domain.InvasionPolicy{
		Archetype:         p.Archetype,
		InvasionEnabled:   p.Params.InvasionEnabled,
		LevelRangeEnabled: p.Params.LevelRangeEnabled,
		WeaponTierEnabled: p.Params.WeaponTierEnabled,
	}, nil
}

// CoopPolicy lê o perfil e monta a política de co-op (spec 007). CoopEnabled =
// params.enabledRoles contém "coop". Arquétipo != host_world_dropin devolve a
// policy com o Archetype preenchido (SupportsCoop = false) — o service decide
// ErrCoopNotSupported. Rede/404/parse → ErrProfileUnavailable.
func (c *Client) CoopPolicy(ctx context.Context, gameID string) (domain.CoopPolicy, error) {
	p, err := c.getProfile(ctx, gameID)
	if err != nil {
		return domain.CoopPolicy{}, err
	}
	coopEnabled := false
	for _, role := range p.Params.EnabledRoles {
		if role == roleCoop {
			coopEnabled = true
			break
		}
	}
	return domain.CoopPolicy{
		Archetype:         p.Archetype,
		CoopEnabled:       coopEnabled,
		LevelRangeEnabled: p.Params.LevelRangeEnabled,
		WeaponTierEnabled: p.Params.WeaponTierEnabled,
	}, nil
}

// Format lê o perfil do jogo e deriva a formação do match (tamanho + estrutura
// + composição de lados). Qualquer falha (jogo/perfil ausente, arquétipo !=
// queue_arena, tamanho inválido, rede) vira ErrProfileUnavailable — o ticket
// não deve entrar na fila.
func (c *Client) Format(ctx context.Context, gameID string) (domain.MatchFormat, error) {
	p, err := c.getProfile(ctx, gameID)
	if err != nil {
		return domain.MatchFormat{}, err
	}
	if p.Archetype != archetypeQueueArena {
		return domain.MatchFormat{}, fmt.Errorf("registryclient: archetype %q não suportado: %w", p.Archetype, domain.ErrProfileUnavailable)
	}

	format := domain.MatchFormat{Structure: p.Params.Structure}
	switch p.Params.Structure {
	case domain.StructureFFA:
		format.Size = p.Params.PlayerCount
	case domain.StructureTeams:
		format.TeamCount = p.Params.TeamCount
		format.TeamSize = p.Params.TeamSize
		format.Size = p.Params.TeamCount * p.Params.TeamSize
	}
	if format.Size <= 0 {
		return domain.MatchFormat{}, fmt.Errorf("registryclient: tamanho inválido (%d) para structure %q: %w",
			format.Size, p.Params.Structure, domain.ErrProfileUnavailable)
	}
	return format, nil
}
