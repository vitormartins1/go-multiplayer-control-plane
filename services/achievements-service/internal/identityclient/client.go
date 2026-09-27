// Package identityclient lê do identity o SteamID do jogador (spec
// identity/006) — adapter de saída da Port domain.SteamIDResolver (spec 019).
package identityclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/achievements-service/internal/domain"
)

// Client chama o /admin do identity com a internal key (ADR-0018).
type Client struct {
	baseURL string
	key     string
	http    *http.Client
}

var _ domain.SteamIDResolver = (*Client)(nil)

// New cria o cliente. hc nil usa um http.Client com timeout de 5s.
func New(baseURL, internalKey string, hc *http.Client) *Client {
	if hc == nil {
		hc = &http.Client{Timeout: 5 * time.Second}
	}
	return &Client{baseURL: baseURL, key: internalKey, http: hc}
}

// SteamID devolve o SteamID64 do jogador. ErrNoSteamAccount se ele não tiver
// vínculo Steam ou se a conta não existir (nada a espelhar em nenhum dos dois).
func (c *Client) SteamID(ctx context.Context, playerID string) (string, error) {
	u := c.baseURL + "/admin/players/" + url.PathEscape(playerID) + "/external-identities"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", fmt.Errorf("identityclient: steam id: %w", err)
	}
	req.Header.Set("X-Internal-Key", c.key)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("identityclient: steam id: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		// só o "jogador não existe" do identity é definitivo; um 404 de rota
		// (identity antigo sem a spec 006, gateway) é transitório — senão todo
		// desbloqueio pendente viraria skipped para sempre.
		var body struct {
			Error string `json:"error"`
		}
		if json.NewDecoder(io.LimitReader(resp.Body, 4<<10)).Decode(&body) == nil && body.Error == "player_not_found" {
			return "", domain.ErrNoSteamAccount
		}
		return "", fmt.Errorf("identityclient: steam id: http 404 (rota?)")
	default:
		return "", fmt.Errorf("identityclient: steam id: http %d", resp.StatusCode)
	}
	var body struct {
		Identities []struct {
			Provider   string `json:"provider"`
			ExternalID string `json:"externalId"`
		} `json:"identities"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&body); err != nil {
		return "", fmt.Errorf("identityclient: steam id: decode: %w", err)
	}
	for _, id := range body.Identities {
		if id.Provider == "steam" && id.ExternalID != "" {
			return id.ExternalID, nil
		}
	}
	return "", domain.ErrNoSteamAccount
}
