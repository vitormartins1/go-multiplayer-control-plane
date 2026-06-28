# Tasks: 001-auth-core
**Plan:** specs/001-auth-core/plan.md
**Data:** 2026-06-27

> Ordem por dependência: módulo → domain → token → service → repository → transport → wire-up.
> Testes acompanham cada camada (não ficam para o fim). Marcar [x] conforme implementa.

## Fase 0 — Módulo e base compartilhada
- [ ] Criar `go.mod` (módulo `github.com/c130n/game-backend`, Go 1.22)
- [ ] `shared/logger/logger.go` — logger slog JSON com nome do serviço (ADR-0006)
- [ ] `shared/config/config.go` — leitura de env: PORT, DSN do Postgres, chave Ed25519

## Fase 1 — Domain
- [ ] `internal/domain/account.go` — struct `Account` + Ports: `AuthService`, `AccountRepository`, `TokenIssuer`
- [ ] `internal/domain/errors.go` — `ErrInvalidHandle`, `ErrHandleTaken`, `ErrWeakPassword`, `ErrInvalidCredentials`

## Fase 2 — Token issuer (Ed25519 + JWKS)
- [ ] `internal/token/ed25519_issuer.go` — carregar/gerar chave; `Sign(subject, claims...)` → JWT EdDSA (golang-jwt) com `sub/iat/exp/kid` + claims extras (ex: `anon`, para a 002); `JWKS()` → JSON do JWK público; `kid` derivado da chave pública
- [ ] `internal/token/ed25519_issuer_test.go` — token assinado é verificável com a pública; token expira; JWKS contém `kid` e nunca a chave privada

## Fase 3 — Service
- [ ] `internal/service/auth_service.go` — `Register` (validar regras da spec → bcrypt hash → `repo.Create`); `Authenticate` (`repo.FindByHandle` → bcrypt compare, com compare "dummy" quando a conta não existe → `token.Sign`)
- [ ] `internal/service/auth_service_test.go` — unit tests cobrindo TODOS os Casos de Teste Obrigatórios da spec (mocks de `AccountRepository` e `TokenIssuer`):
  - register válido (sem vazar senha/hash); handle inválido; senha fraca; handle duplicado (ci)
  - login correto (JWT com sub/exp/kid); senha errada e handle inexistente → mesmo `ErrInvalidCredentials`

## Fase 4 — Repository
- [ ] `migrations/001_create_accounts.sql` — tabela `accounts` (id, handle único case-insensitive, password_hash, created_at)
- [ ] `internal/repository/postgres_account.go` — `Create`, `FindByHandle` (case-insensitive) com pgx
- [ ] `internal/repository/postgres_account_test.go` — integration tests com testcontainers: criar conta; unicidade de handle case-insensitive; buscar por handle

## Fase 5 — Transport
- [ ] `internal/transport/http_handler.go` — `POST /accounts`, `POST /sessions`, `GET /.well-known/jwks.json`; mapear erros de domínio → HTTP (400 InvalidHandle/WeakPassword, 401 InvalidCredentials, 409 HandleTaken)
- [ ] `internal/transport/http_handler_test.go` — httptest: caminho feliz dos 3 endpoints + mapeamento de cada erro → status

## Fase 6 — Wire-up e verificação
- [ ] `cmd/server/main.go` — wire-up (config, logger, pgxpool, repo, token issuer, service, handler) + HTTP server com graceful shutdown
- [ ] `services/identity-service/Dockerfile` — build multi-stage (contexto = raiz; módulo único)
- [ ] `infra/docker/docker-compose.yml` — Postgres + identity-service (aplicar migrations no setup local)
- [ ] Confirmar `go test ./... -race -cover` e `go build ./...` passando; todos os Casos de Teste Obrigatórios da spec verdes
