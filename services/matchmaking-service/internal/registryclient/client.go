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

	"github.com/vitormartins1/go-multiplayer-control-plane/services/matchmaking-service/internal/domain"
)

var (
	_ domain.ProfileProvider        = (*Client)(nil)
	_ domain.InvasionPolicyProvider = (*Client)(nil)
	_ domain.CoopPolicyProvider     = (*Client)(nil)
)

// roleCoop é o papel cujo habilitação (em params.enabledRoles) o quadro de sinais
// exige (spec 007). Vocabulário do game-registry.
const roleCoop = "coop"

// Arquétipos cuja formação o matchmaking sabe derivar via Format (ADR-0023).
// Vocabulário do game-registry. host_world_dropin não entra aqui: não usa fila
// (usa invasão/sinais — specs 006/007), então Format o rejeita.
const (
	archetypeQueueArena   = "queue_arena"
	archetypeBattleRoyale = "battle_royale"
	archetypeCoopMission  = "coop_mission"
)

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
		// battle_royale (spec 007): alvo do lobby; TeamSize (acima) é o squad.
		LobbySize int `json:"lobbySize"`
		// host_world_dropin (spec 006/007): flags que ligam/desligam cada filtro.
		// Ausentes em queue_arena (ficam false). enabledRoles lista os papéis
		// habilitados (coop/invader) — o co-op (spec 007) exige "coop".
		InvasionEnabled   bool     `json:"invasionEnabled"`
		LevelRangeEnabled bool     `json:"levelRangeEnabled"`
		WeaponTierEnabled bool     `json:"weaponTierEnabled"`
		EnabledRoles      []string `json:"enabledRoles"`
	} `json:"params"`
}

// getProfile faz o GET do perfil do MODO (ADR-0024: URL da coleção do M0; o
// alias legado saiu) e decodifica. Erro de rede/status/parse vira
// ErrProfileUnavailable — compartilhado por Format, InvasionPolicy e CoopPolicy.
func (c *Client) getProfile(ctx context.Context, gameID, modeID string) (profileResponse, error) {
	u := c.baseURL + "/games/" + url.PathEscape(gameID) + "/multiplayer-profiles/" + url.PathEscape(modeID)
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
func (c *Client) InvasionPolicy(ctx context.Context, gameID, modeID string) (domain.InvasionPolicy, error) {
	p, err := c.getProfile(ctx, gameID, modeID)
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
func (c *Client) CoopPolicy(ctx context.Context, gameID, modeID string) (domain.CoopPolicy, error) {
	p, err := c.getProfile(ctx, gameID, modeID)
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
// + composição de lados). Qualquer falha (jogo/perfil ausente, arquétipo sem
// fila, tamanho inválido, rede) vira ErrProfileUnavailable — o ticket não deve
// entrar na fila. Serve queue_arena (ffa/teams), battle_royale (ffa/squads) e
// coop_mission (um único time = teams com TeamCount=1); host_world_dropin não
// usa fila.
func (c *Client) Format(ctx context.Context, gameID, modeID string) (domain.MatchFormat, error) {
	p, err := c.getProfile(ctx, gameID, modeID)
	if err != nil {
		return domain.MatchFormat{}, err
	}

	var format domain.MatchFormat
	switch p.Archetype {
	case archetypeQueueArena:
		format = domain.MatchFormat{Structure: p.Params.Structure}
		switch p.Params.Structure {
		case domain.StructureFFA:
			format.Size = p.Params.PlayerCount
		case domain.StructureTeams:
			format.TeamCount = p.Params.TeamCount
			format.TeamSize = p.Params.TeamSize
			format.Size = p.Params.TeamCount * p.Params.TeamSize
		}
	case archetypeBattleRoyale:
		if p.Params.TeamSize <= 0 {
			return domain.MatchFormat{}, fmt.Errorf("registryclient: teamSize inválido (%d) para battle_royale: %w",
				p.Params.TeamSize, domain.ErrProfileUnavailable)
		}
		// teamSize 1 = solo → ffa (colocação por jogador, sem team_id); teamSize
		// >= 2 = duo/squad → squads (partição ragged na composição).
		if p.Params.TeamSize == 1 {
			format = domain.MatchFormat{Structure: domain.StructureFFA, Size: p.Params.LobbySize}
		} else {
			format = domain.MatchFormat{Structure: domain.StructureSquads, Size: p.Params.LobbySize, TeamSize: p.Params.TeamSize}
		}
	case archetypeCoopMission:
		// co-op = um único time contra o ambiente (ficha 04): caso degenerado do
		// NvN (TeamCount=1). composeTeams/StructureTeams já produz o time único.
		if p.Params.TeamSize < 2 {
			return domain.MatchFormat{}, fmt.Errorf("registryclient: teamSize inválido (%d) para coop_mission: %w",
				p.Params.TeamSize, domain.ErrProfileUnavailable)
		}
		format = domain.MatchFormat{Structure: domain.StructureTeams, TeamCount: 1, TeamSize: p.Params.TeamSize, Size: p.Params.TeamSize}
	default:
		return domain.MatchFormat{}, fmt.Errorf("registryclient: archetype %q não suportado: %w", p.Archetype, domain.ErrProfileUnavailable)
	}

	if format.Size <= 0 {
		return domain.MatchFormat{}, fmt.Errorf("registryclient: tamanho inválido (%d) para archetype %q: %w",
			format.Size, p.Archetype, domain.ErrProfileUnavailable)
	}
	return format, nil
}
