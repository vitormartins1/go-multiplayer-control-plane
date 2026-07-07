// Package upstream fornece helpers compartilhados para os adapters de saída do
// admin-bff: client HTTP padrão, extração de código de erro e classificação de
// respostas upstream (2xx / 4xx / outro).
package upstream

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/admin-bff/internal/domain"
)

// DefaultClient retorna um *http.Client com timeout de 5 s — usar em New()
// quando o caller não injeta client customizado.
func DefaultClient() *http.Client {
	return &http.Client{Timeout: 5 * time.Second}
}

// ErrorCode extrai o campo "error" do corpo JSON da resposta. O body deve estar
// aberto; fechar é responsabilidade do caller.
func ErrorCode(resp *http.Response) string {
	var body struct {
		Error string `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return body.Error
}

// Do executa a request e classifica a resposta:
//   - 2xx → devolve resp (caller fecha o body)
//   - 4xx → fecha body, devolve *domain.UpstreamError
//   - conn/5xx → fecha body, devolve unavailable wrappado
func Do(client *http.Client, req *http.Request, unavailable error) (*http.Response, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", unavailable, err)
	}
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return resp, nil
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		code := ErrorCode(resp)
		_ = resp.Body.Close()
		return nil, &domain.UpstreamError{Status: resp.StatusCode, Code: code}
	default:
		_ = resp.Body.Close()
		return nil, fmt.Errorf("%w: status %d", unavailable, resp.StatusCode)
	}
}
