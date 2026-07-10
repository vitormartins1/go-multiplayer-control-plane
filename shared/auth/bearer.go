package auth

import "strings"

// bearerPrefix é o prefixo do esquema Bearer (RFC 6750); o nome do esquema é
// case-insensitive (RFC 7230 §3.2), daí o EqualFold só no prefixo — o token
// em si é comparado exato.
const bearerPrefix = "Bearer "

// BearerToken extrai o token de um valor bruto de Authorization no formato
// "Bearer <token>". Devolve "" se o valor não seguir o esquema (sem prefixo,
// ou token vazio após o prefixo). Recebe o valor já lido (não o header/
// metadata inteiro) pra servir tanto o transport HTTP quanto os interceptors
// gRPC — mesma extração, achado no /code-review: reimplementada
// identicamente em 5 serviços antes de centralizar aqui.
func BearerToken(value string) string {
	if len(value) > len(bearerPrefix) && strings.EqualFold(value[:len(bearerPrefix)], bearerPrefix) {
		return value[len(bearerPrefix):]
	}
	return ""
}
