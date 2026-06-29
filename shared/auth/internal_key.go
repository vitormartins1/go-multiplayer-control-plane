package auth

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// InternalKeyHeader é o header de autenticação serviço-a-serviço (ADR-0018).
const InternalKeyHeader = "X-Internal-Key"

// RequireInternalKey devolve um middleware que exige o header X-Internal-Key
// igual a `key` nas rotas cujo path começa com algum dos `prefixes` (control
// plane interno — ADR-0018). Rotas fora dos prefixes (ex.: /healthz, /metrics)
// passam livres, para o Prometheus continuar raspando. Falha closed: `key`
// vazia nega as rotas protegidas (força a configuração).
func RequireInternalKey(key string, prefixes ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if matchesPrefix(r.URL.Path, prefixes) && !validInternalKey(r, key) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func matchesPrefix(path string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

func validInternalKey(r *http.Request, key string) bool {
	if key == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(r.Header.Get(InternalKeyHeader)), []byte(key)) == 1
}

// InternalKeyTransport envolve um RoundTripper injetando o X-Internal-Key nas
// requisições de saída (callers serviço-a-serviço, ex.: admin-bff, session).
func InternalKeyTransport(key string, base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return internalKeyTransport{key: key, base: base}
}

type internalKeyTransport struct {
	key  string
	base http.RoundTripper
}

func (t internalKeyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r2 := r.Clone(r.Context())
	r2.Header.Set(InternalKeyHeader, t.key)
	return t.base.RoundTrip(r2)
}
