// Package steam valida tickets de login da Steam na Web API
// (ISteamUserAuth/AuthenticateUserTicket) — adapter de saída da Port
// domain.ExternalIdentityVerifier (spec 005-steam-login).
//
// Fluxo: o cliente do jogo pede um ticket com
// ISteamUser::GetAuthTicketForWebApi("<identity>") e manda o ticket (hex) ao
// identity; aqui ele é trocado pelo SteamID64 do jogador.
package steam

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/identity-service/internal/domain"
)

// DefaultBaseURL é o host da Web API de publisher (exige a publisher key).
const DefaultBaseURL = "https://partner.steam-api.com"

// maxTicketHexLen limita o ticket antes de ir à rede (tickets da Steam têm
// algumas centenas de bytes; o teto evita repassar lixo arbitrário).
const maxTicketHexLen = 4096

// Config configura o verificador.
type Config struct {
	BaseURL  string // DefaultBaseURL se vazio
	APIKey   string // publisher Web API key (STEAM_WEB_API_KEY)
	AppID    string // app id do jogo na Steam (STEAM_APP_ID)
	Identity string // identidade passada ao GetAuthTicketForWebApi (STEAM_TICKET_IDENTITY)
}

// Verifier valida tickets na Web API da Steam.
type Verifier struct {
	cfg    Config
	client *http.Client
}

var _ domain.ExternalIdentityVerifier = (*Verifier)(nil)

// New cria o verificador. client nil usa um http.Client com timeout de 5s.
func New(cfg Config, client *http.Client) *Verifier {
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &Verifier{cfg: cfg, client: client}
}

// Provider é o nome do provedor na rota POST /sessions/{provider}.
func (v *Verifier) Provider() string { return "steam" }

// response é o envelope da AuthenticateUserTicket: sucesso em params, falha
// em error (a Steam responde 200 nos dois casos).
type response struct {
	Response struct {
		Params *struct {
			Result          string `json:"result"`
			SteamID         string `json:"steamid"`
			OwnerSteamID    string `json:"ownersteamid"`
			VACBanned       bool   `json:"vacbanned"`
			PublisherBanned bool   `json:"publisherbanned"`
		} `json:"params"`
		Error *struct {
			Code int    `json:"errorcode"`
			Desc string `json:"errordesc"`
		} `json:"error"`
	} `json:"response"`
}

// Verify troca o ticket pelo SteamID64. Ticket malformado ou recusado →
// ErrInvalidTicket; publisher ban → ErrExternalBanned (VAC ban é por jogo e
// não bloqueia aqui); Steam fora do ar ou chave recusada →
// ErrProviderUnavailable.
func (v *Verifier) Verify(ctx context.Context, ticket string) (domain.ExternalIdentity, error) {
	if ticket == "" || len(ticket) > maxTicketHexLen {
		return domain.ExternalIdentity{}, domain.ErrInvalidTicket
	}
	if _, err := hex.DecodeString(ticket); err != nil {
		return domain.ExternalIdentity{}, domain.ErrInvalidTicket
	}

	q := url.Values{}
	q.Set("key", v.cfg.APIKey)
	q.Set("appid", v.cfg.AppID)
	q.Set("ticket", ticket)
	if v.cfg.Identity != "" {
		q.Set("identity", v.cfg.Identity)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		v.cfg.BaseURL+"/ISteamUserAuth/AuthenticateUserTicket/v1/?"+q.Encode(), nil)
	if err != nil {
		return domain.ExternalIdentity{}, fmt.Errorf("steam: request: %w", err)
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return domain.ExternalIdentity{}, fmt.Errorf("steam: %w: %v", domain.ErrProviderUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		// 403 = publisher key recusada (configuração); 5xx = Steam fora.
		return domain.ExternalIdentity{}, fmt.Errorf("steam: %w: status %d", domain.ErrProviderUnavailable, resp.StatusCode)
	}

	var body response
	if err := json.NewDecoder(http.MaxBytesReader(nil, resp.Body, 64<<10)).Decode(&body); err != nil {
		return domain.ExternalIdentity{}, fmt.Errorf("steam: %w: decode: %v", domain.ErrProviderUnavailable, err)
	}
	p := body.Response.Params
	switch {
	case body.Response.Error != nil || p == nil:
		return domain.ExternalIdentity{}, domain.ErrInvalidTicket
	case p.Result != "OK" || p.SteamID == "":
		return domain.ExternalIdentity{}, domain.ErrInvalidTicket
	case p.PublisherBanned:
		return domain.ExternalIdentity{}, domain.ErrExternalBanned
	}
	return domain.ExternalIdentity{Provider: v.Provider(), ExternalID: p.SteamID}, nil
}
