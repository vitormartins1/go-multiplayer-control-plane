# Tasks: 002-anonymous-login
**Plan:** specs/002-anonymous-login/plan.md
**Data:** 2026-06-27

> Depende do 001-auth-core implementado (modifica service e transport).
> Marcar [x] conforme implementa.

## Fase 1 — Service
- [ ] Adicionar `LoginAnonymous(ctx) (string, error)` à Port `AuthService` em `internal/domain/account.go`
- [ ] Implementar `LoginAnonymous` em `internal/service/auth_service.go` (gera `guest-<uuid>`, chama `token.Sign` com claim `anon: true`)
- [ ] Unit tests em `internal/service/auth_service_test.go`:
  - token contém `sub` no formato `guest-...` e claim `anon: true`
  - dois logins anônimos geram subjects distintos
  - nada é persistido (repositório não é chamado)

## Fase 2 — Transport
- [ ] Adicionar `POST /sessions/anonymous` em `internal/transport/http_handler.go`
- [ ] httptest em `internal/transport/http_handler_test.go`: 200 + JWT; token verificável com a chave pública do JWKS (001)

## Fase 3 — Verificação
- [ ] `go test ./... -race` passando; todos os Casos de Teste Obrigatórios da spec 002 verdes
