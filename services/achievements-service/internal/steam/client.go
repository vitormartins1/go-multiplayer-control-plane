// Package steam marca conquistas na Steam pela Web API de publisher
// (ISteamUserStats/SetUserStatsForGame) — adapter de saída da Port
// domain.SteamAchievementSetter (spec 019).
package steam

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/achievements-service/internal/domain"
)

// DefaultBaseURL é o host da Web API de publisher (exige a publisher key).
const DefaultBaseURL = "https://partner.steam-api.com"

// Config configura o cliente.
type Config struct {
	BaseURL string // DefaultBaseURL se vazio
	APIKey  string // publisher Web API key (STEAM_WEB_API_KEY)
	AppID   string // app id do jogo na Steam (STEAM_APP_ID)
}

// Client chama a Web API da Steam.
type Client struct {
	cfg  Config
	http *http.Client
}

var _ domain.SteamAchievementSetter = (*Client)(nil)

// New cria o cliente. hc nil usa um http.Client com timeout de 10s.
func New(cfg Config, hc *http.Client) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{cfg: cfg, http: hc}
}

// SetAchievement marca a conquista apiName do jogador steamID
// (SetUserStatsForGame com value 1). Sucesso só com response.result == 1;
// qualquer outra resposta vira erro (o service reagenda).
func (c *Client) SetAchievement(ctx context.Context, steamID, apiName string) error {
	form := url.Values{
		"key":      {c.cfg.APIKey},
		"steamid":  {steamID},
		"appid":    {c.cfg.AppID},
		"count":    {"1"},
		"name[0]":  {apiName},
		"value[0]": {"1"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.cfg.BaseURL+"/ISteamUserStats/SetUserStatsForGame/v1/", strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("steam: set achievement: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("steam: set achievement: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		// sem o corpo no erro: pode ecoar a chave
		return fmt.Errorf("steam: set achievement: http %d", resp.StatusCode)
	}
	var body struct {
		Response struct {
			Result int `json:"result"`
		} `json:"response"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&body); err != nil {
		return fmt.Errorf("steam: set achievement: decode: %w", err)
	}
	if body.Response.Result != 1 {
		return fmt.Errorf("steam: set achievement: result %d", body.Response.Result)
	}
	return nil
}
