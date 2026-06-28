package middleware

import (
	"net/http"
	"strings"

	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// RouteSpanName renomeia o span do request (criado pelo otelhttp) para o
// pattern casado pelo ServeMux — ex. "POST /accounts" no lugar de "http.server"
// — e anexa o atributo http.route. Precisa envolver o mux por fora: o pattern
// só aparece em r.Pattern depois que o mux casa a rota (o net/http o seta no
// próprio request, então fica visível ao retornar do next). Em rota sem span
// ativo (ex.: filtrada pelo otelhttp), é no-op. Ver ADR-0014.
func RouteSpanName(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)

		if r.Pattern == "" {
			return
		}
		span := trace.SpanFromContext(r.Context())
		if !span.IsRecording() {
			return
		}
		span.SetName(r.Pattern)
		if path := routePath(r.Pattern); path != "" {
			span.SetAttributes(semconv.HTTPRouteKey.String(path))
		}
	})
}

// routePath extrai o template de path de um pattern de ServeMux:
// "POST /accounts" -> "/accounts"; "/accounts" -> "/accounts".
func routePath(pattern string) string {
	if i := strings.IndexByte(pattern, '/'); i >= 0 {
		return pattern[i:]
	}
	return ""
}
