package auth

import "testing"

// TestBearerToken cobre a extração centralizada (achado no /code-review:
// reimplementada identicamente em 5 serviços antes de virar shared/auth).
func TestBearerToken(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  string
	}{
		{"prefixo padrão", "Bearer abc123", "abc123"},
		{"prefixo case-insensitive", "bearer abc123", "abc123"},
		{"prefixo misto", "BeArEr abc123", "abc123"},
		{"vazio", "", ""},
		{"sem prefixo", "abc123", ""},
		{"só o prefixo, sem token", "Bearer ", ""},
		{"prefixo sem espaço", "Bearerabc123", ""},
		{"outro esquema", "Basic dXNlcjpwYXNz", ""},
		{"token com espaços internos preservado", "Bearer a b c", "a b c"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := BearerToken(c.value); got != c.want {
				t.Fatalf("BearerToken(%q) = %q, quer %q", c.value, got, c.want)
			}
		})
	}
}
