// Package cloudsave é o adapter de saída que lê a API admin do
// cloud-save-service (save por jogo+jogador, ADR-0017) e satisfaz
// domain.CloudSaveReader. Ausência do save (404) vira domain.UpstreamError(404)
// (repassado); indisponibilidade vira domain.ErrCloudSaveUnavailable.
package cloudsave

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/admin-bff/internal/domain"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/admin-bff/internal/upstream"
)

var _ domain.CloudSaveReader = (*Client)(nil)

// Client lê o cloud-save-service em baseURL (ex.: http://cloud-save-service:8080).
type Client struct {
	baseURL string
	http    *http.Client
}

// New cria o client. O httpClient permite injetar timeout/transport (otelhttp +
// internal key).
func New(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = upstream.DefaultClient()
	}
	return &Client{baseURL: baseURL, http: httpClient}
}

// saveResponse é o shape de GET /admin/games/{g}/players/{p}/save.
type saveResponse struct {
	ContentType string    `json:"contentType"`
	Size        int       `json:"size"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Blob        string    `json:"blob"`
}

// ForPlayer lê o save de um jogador num jogo. 404 (sem save) vira
// UpstreamError(404), repassado pelo transport.
func (c *Client) ForPlayer(ctx context.Context, gameID, playerID string) (domain.CloudSave, error) {
	u := c.baseURL + "/admin/games/" + url.PathEscape(gameID) + "/players/" + url.PathEscape(playerID) + "/save"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	resp, err := upstream.Do(c.http, req, domain.ErrCloudSaveUnavailable)
	if err != nil {
		return domain.CloudSave{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	var body saveResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return domain.CloudSave{}, fmt.Errorf("cloudsave: decode: %w", err)
	}
	return domain.CloudSave{
		ContentType: body.ContentType,
		Size:        body.Size,
		UpdatedAt:   body.UpdatedAt,
		Blob:        body.Blob,
	}, nil
}

