# Plan: 001-auth-core
**Spec:** specs/001-auth-core/spec.md
**Data:** 2026-06-27

## Abordagem técnica

API REST com `net/http` (ServeMux do Go 1.22, method patterns). Arquitetura hexagonal (ADR-0002): `domain` define `Account` e as Ports; `service` tem a regra de negócio; `repository` persiste em Postgres; `transport` traduz HTTP. O par de chaves Ed25519 e o JWKS ficam num adapter de saída (`internal/token`).

Fluxo: `Register` valida → hasheia a senha → persiste; `Authenticate` busca por handle → compara hash → emite JWT assinado. O JWKS expõe a chave pública para verificação local pelos consumidores.

## Compute e banco (ADR-0005)

- **Banco: PostgreSQL** (relacional, adequado para contas com unicidade de handle). É o default local do projeto, então não há desvio que justifique ADR próprio.
- **Local:** Postgres em container (docker-compose); testes de integração com `testcontainers-go` (Postgres real e descartável).
- **Cloud/deploy:** fora do escopo desta feature — backlog (task #10).
- **Compute:** decisão de deploy adiada (provável Lambda, ADR-0009); não afeta o código desta feature.

## Arquivos a criar ou modificar

- `go.mod` — módulo único, dependências
- `services/identity-service/internal/domain/account.go` — struct Account + Ports (AuthService, AccountRepository, TokenIssuer)
- `services/identity-service/internal/domain/errors.go` — ErrInvalidHandle, ErrHandleTaken, ErrWeakPassword, ErrInvalidCredentials
- `services/identity-service/internal/service/auth_service.go` — Register, Authenticate
- `services/identity-service/internal/service/auth_service_test.go` — unit tests (mocks)
- `services/identity-service/internal/repository/postgres_account.go` — AccountRepository em pgx
- `services/identity-service/internal/repository/postgres_account_test.go` — integration tests (testcontainers)
- `services/identity-service/internal/token/ed25519_issuer.go` — TokenIssuer (assina JWT EdDSA + monta JWKS)
- `services/identity-service/internal/token/ed25519_issuer_test.go` — sign/verify + JWKS
- `services/identity-service/internal/transport/http_handler.go` — handlers + mapeamento erro→HTTP
- `services/identity-service/internal/transport/http_handler_test.go` — httptest
- `services/identity-service/migrations/001_create_accounts.sql`
- `services/identity-service/cmd/server/main.go` — wire-up + graceful shutdown
- `shared/logger`, `shared/config` — base (logger slog, leitura de env)
- `infra/docker/docker-compose.yml` — Postgres para dev local

## Dependências e libs

- `github.com/jackc/pgx/v5` — driver/pool PostgreSQL (stack do projeto)
- `github.com/golang-jwt/jwt/v5` — emissão/verificação de JWT (suporta EdDSA); `crypto/ed25519` (stdlib) para o par de chaves
- JWKS montado à mão (JSON pequeno: `kty=OKP, crv=Ed25519, x=<pub base64url>, kid`) — sem lib extra, e didático
- `golang.org/x/crypto/bcrypt` — hash de senha
- `github.com/google/uuid` — IDs de conta
- `github.com/golang-migrate/migrate/v4` — aplicar migrations versionadas
- Testes: `github.com/stretchr/testify`, `github.com/testcontainers/testcontainers-go`

## Decisões técnicas e trade-offs

- **bcrypt** para hash de senha: simples, testado, um único parâmetro (cost). Alternativa mais moderna seria **argon2id** (mais forte, mas exige tunar memória/tempo). Escolho bcrypt pela simplicidade; trocável depois sem afetar a spec.
- **Chave Ed25519 (dev):** carregada de config (env, PEM). Se ausente em dev, gera um par efêmero no boot e loga um aviso. Produção: chave vinda de secret manager (backlog). O `kid` é derivado do hash da chave pública (habilita rotação futura).
- **Não vazar existência do handle:** no `Authenticate`, quando o handle não existe, ainda executar uma comparação bcrypt "dummy" para evitar oráculo de timing — mesma resposta e tempo aproximado de senha incorreta.
- **JWKS à mão** em vez de uma lib de JWK: menos dependência e melhor para aprender o formato; o JSON é pequeno e estável.
- **golang-jwt vs jwx:** golang-jwt é mais simples/difundido; como o JWKS é montado à mão, não preciso do jwx. Trade: se o JWKS crescer (rotação, múltiplas chaves), reavaliar jwx.

## Ordem de implementação

1. `domain` (Account, Ports, errors) — base sem dependências
2. `internal/token` (Ed25519 sign + JWKS) + testes — pura, destrava o service
3. `service` (Register, Authenticate) + unit tests (mock repo e token issuer)
4. `repository` Postgres + migration + integration tests (testcontainers)
5. `transport` (handlers, erro→HTTP) + httptest
6. wire-up em `cmd/server/main.go` + Postgres no docker-compose
7. `go test ./... -race` e `go build ./...` passando

## Riscos e pontos de atenção

- **testcontainers exige Docker rodando** na máquina de dev/CI.
- **Chave efêmera em dev:** tokens deixam de validar após restart se a chave for regenerada — para dev contínuo, fixar a chave via env.
- **Timing oracle** no login: garantir a comparação dummy (acima).
- **Observabilidade:** o serviço já nasce com slog (ADR-0006); métricas Prometheus entram na task #5 — manter os pontos de instrumentação previstos no handler/middleware.
