// Package prometheus é o adapter de saída que lê a HTTP API do Prometheus
// (instant query) e satisfaz domain.MetricsSource (ADR-0002/0016). Usa HTTP cru
// + encoding/json — sem a client lib do Prometheus (deps mínimas).
package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/vitormartins1/go-game-backend/services/admin-bff/internal/domain"
)

var _ domain.MetricsSource = (*Client)(nil)

// Client consulta o Prometheus em baseURL (ex.: http://prometheus:9090).
type Client struct {
	baseURL string
	http    *http.Client
}

// New cria o client. O httpClient permite injetar timeout/transport (otelhttp).
func New(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}
	return &Client{baseURL: baseURL, http: httpClient}
}

// queryResponse é o shape de /api/v1/query (resultType "vector").
type queryResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  [2]any            `json:"value"` // [ <ts float>, "<valor string>" ]
		} `json:"result"`
	} `json:"data"`
}

// Query roda uma instant query. Falha de conexão/HTTP vira ErrMetricsUnavailable
// (para o service distinguir "fonte fora" de "erro de dados").
func (c *Client) Query(ctx context.Context, promQL string) ([]domain.Sample, error) {
	u := c.baseURL + "/api/v1/query?" + url.Values{"query": {promQL}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("prometheus: build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrMetricsUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", domain.ErrMetricsUnavailable, resp.StatusCode)
	}

	var qr queryResponse
	if err := json.NewDecoder(resp.Body).Decode(&qr); err != nil {
		return nil, fmt.Errorf("prometheus: decode: %w", err)
	}
	if qr.Status != "success" {
		return nil, fmt.Errorf("prometheus: query status %q", qr.Status)
	}

	out := make([]domain.Sample, 0, len(qr.Data.Result))
	for _, r := range qr.Data.Result {
		// value[1] é o número como string (formato do Prometheus).
		raw, _ := r.Value[1].(string)
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, fmt.Errorf("prometheus: parse value %q: %w", raw, err)
		}
		out = append(out, domain.Sample{Labels: r.Metric, Value: v})
	}
	return out, nil
}
