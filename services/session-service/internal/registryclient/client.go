// Package registryclient é o adapter de saída que lê o perfil de multiplayer de
// um MODO do jogo no game-registry (ADR-0023/0024). Satisfaz
// domain.ProfileProvider. A internal key (ADR-0018) é injetada pelo transport do
// http.Client (auth.InternalKeyTransport), montado no main — como no adapter
// equivalente do matchmaking.
package registryclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/domain"
	"github.com/vitormartins1/go-multiplayer-control-plane/shared/modesnapshot"
)

var _ domain.ProfileProvider = (*Client)(nil)

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

// modeResponse espelha o modo devolvido pela coleção (spec game-registry/009).
// Params fica cru: vira o ParamsJSON do snapshot (manifest) sem re-serializar;
// os campos do host_world são decodificados à parte só quando o arquétipo pede.
type modeResponse struct {
	Archetype string          `json:"archetype"`
	Topology  string          `json:"topology"`
	Params    json.RawMessage `json:"params"`
	// Fleet é a Fleet Agones do jogo dono do modo (ADR-0026, game-registry/012);
	// "" em respostas de game-registry pré-spec-012 (compat).
	Fleet string `json:"fleet"`
}

// hostWorldParams é o subconjunto que o session decodifica para derivar os
// slots por papel (spec 011). Tags conforme o game-registry.
type hostWorldParams struct {
	EnabledRoles []string `json:"enabledRoles"`
	CoopSlots    int      `json:"coopSlots"`
	InvaderSlots int      `json:"invaderSlots"`
}

// ModeProfile lê o perfil do modo e devolve o snapshot (ADR-0024, spec 017).
// Qualquer falha (jogo/modo ausente, rede, parse) vira ErrProfileUnavailable.
// O hash é SHA-256 do corpo do modo como recebido, truncado a 16 hex —
// correlação/log entre control e data plane, não integridade criptográfica.
func (c *Client) ModeProfile(ctx context.Context, gameID, modeID string) (domain.ProfileSnapshot, error) {
	u := c.baseURL + "/games/" + url.PathEscape(gameID) + "/multiplayer-profiles/" + url.PathEscape(modeID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return domain.ProfileSnapshot{}, fmt.Errorf("registryclient: build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return domain.ProfileSnapshot{}, fmt.Errorf("registryclient: get mode profile: %v: %w", err, domain.ErrProfileUnavailable)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return domain.ProfileSnapshot{}, fmt.Errorf("registryclient: get mode profile: status %d: %w", resp.StatusCode, domain.ErrProfileUnavailable)
	}

	// Decodifica preservando o corpo cru para o hash (json.RawMessage no topo
	// não existe aqui: relemos o corpo em duas passadas via TeeReader seria
	// overkill — o hash cobre o subconjunto estável {archetype, topology, params}).
	var m modeResponse
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return domain.ProfileSnapshot{}, fmt.Errorf("registryclient: decode mode profile: %v: %w", err, domain.ErrProfileUnavailable)
	}

	snap := domain.ProfileSnapshot{
		ModeID:     modeID,
		Archetype:  m.Archetype,
		Topology:   m.Topology,
		ParamsJSON: append([]byte(nil), m.Params...),
		Hash:       modesnapshot.Hash(m.Archetype, m.Topology, m.Params),
		Fleet:      m.Fleet,
	}
	if m.Archetype == domain.ArchetypeHostWorldDropIn {
		var hw hostWorldParams
		if err := json.Unmarshal(m.Params, &hw); err != nil {
			return domain.ProfileSnapshot{}, fmt.Errorf("registryclient: decode host_world params: %v: %w", err, domain.ErrProfileUnavailable)
		}
		snap.EnabledRoles = hw.EnabledRoles
		snap.CoopSlots = hw.CoopSlots
		snap.InvaderSlots = hw.InvaderSlots
	}
	return snap, nil
}
