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

	"github.com/c130n/game-backend/services/admin-bff/internal/domain"
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
		httpClient = &http.Client{Timeout: 5 * time.Second}
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
	resp, err := c.do(req)
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

// do executa a request e classifica a resposta (2xx ok; 4xx -> UpstreamError;
// conn/5xx -> ErrCloudSaveUnavailable). Devolve a resposta só no caminho ok.
func (c *Client) do(req *http.Request) (*http.Response, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrCloudSaveUnavailable, err)
	}
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return resp, nil
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		defer func() { _ = resp.Body.Close() }()
		return nil, &domain.UpstreamError{Status: resp.StatusCode, Code: errorCode(resp)}
	default:
		_ = resp.Body.Close()
		return nil, fmt.Errorf("%w: status %d", domain.ErrCloudSaveUnavailable, resp.StatusCode)
	}
}

func errorCode(resp *http.Response) string {
	var body struct {
		Error string `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return body.Error
}
