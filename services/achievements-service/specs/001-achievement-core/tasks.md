# Tasks: 001-achievement-core
**Plan:** specs/001-achievement-core/plan.md
**Data:** 2026-06-28

> Ordem por dependência: config → domain → auth (JWKS) → service → repository → transport → wire-up/infra → collection.
> Testes acompanham cada camada (não ficam para o fim). Marcar [x] conforme implementa.
> Reaproveita `shared/` (logger, config, middleware, telemetry) — o serviço nasce instrumentado (ADR-0006/0014).

## Fase 0 — Base compartilhada
- [x] `shared/config/config.go` — adicionar `IdentityJWKSURL` (env `IDENTITY_JWKS_URL`)

## Fase 1 — Domain
- [x] `internal/domain/achievement.go` — structs `Achievement` (catálogo: ID, Name, Description, EventType), `Unlock` (PlayerID, AchievementID, UnlockedAt), `PlayerAchievement` (Achievement + Unlocked + UnlockedAt), `Event` (Type) + Ports: `AchievementService`, `AchievementRepository`, `TokenVerifier`
- [x] `internal/domain/errors.go` — `ErrUnauthorized`, `ErrInvalidEvent`

## Fase 2 — Auth (verificação do JWT via JWKS)
- [x] `internal/auth/jwks_verifier.go` — implementa `TokenVerifier`: busca o JWKS do identity (`IdentityJWKSURL`), **lazy + cache `map[kid]ed25519.PublicKey`** com TTL e refetch sob `kid` desconhecido; valida o JWT EdDSA com `golang-jwt/jwt/v5` (assinatura + `exp`), retorna o `sub` como `playerID`
- [x] `internal/auth/jwks_verifier_test.go` — token válido → `playerID`; token expirado → `ErrUnauthorized`; assinatura inválida → `ErrUnauthorized`; `kid` desconhecido → refetch; token ausente/malformado → `ErrUnauthorized` (usa um issuer Ed25519 de teste + JWKS servido por `httptest`)

## Fase 3 — Service
- [x] `internal/service/achievement_service.go` — `ListCatalog`; `ListForPlayer` (catálogo anotado com os unlocks do jogador); `RecordEvent` (valida `Type` → `FindByEventType` → `Unlock` idempotente de cada uma → retorna as **recém-desbloqueadas**)
- [x] `internal/service/achievement_service_test.go` — unit (mock `AchievementRepository`), cobrindo os Casos de Teste Obrigatórios da spec:
  - evento casa uma conquista → desbloqueia e retorna a nova
  - evento repetido → idempotente (lista vazia, `unlockedAt` inalterado)
  - evento sem match → lista vazia
  - evento sem `type` → `ErrInvalidEvent`
  - `ListForPlayer` reflete unlocked + locked

## Fase 4 — Repository
- [x] `migrations/001_create_achievements.sql` — tabela `achievements` (catálogo) + `player_achievements` (PK `(player_id, achievement_id)`, `player_id` TEXT p/ suportar guest) + **seed do catálogo**
- [x] `internal/repository/postgres_achievement.go` — `ListCatalog`, `FindByEventType`, `ListPlayerUnlocks`, `Unlock` via `INSERT ... ON CONFLICT (player_id, achievement_id) DO NOTHING RETURNING` (→ `created bool` atômico)
- [x] `internal/repository/postgres_achievement_test.go` — integration (testcontainers): catálogo carregado por seed; unlock cria; unlock repetido é no-op (`created=false`, `unlockedAt` imutável); listar unlocks por jogador; desbloqueios independentes entre dois jogadores

## Fase 5 — Transport
- [x] `internal/transport/http_handler.go` — **middleware de auth** (extrai `Bearer` → `TokenVerifier.Verify` → injeta `playerID` no contexto; 401 em falha); rotas `POST /events`, `GET /achievements`, `GET /me/achievements`; mapear erros → HTTP (401 `ErrUnauthorized`, 400 `ErrInvalidEvent`); `playerID` sempre do token, nunca do corpo
- [x] `internal/transport/http_handler_test.go` — httptest: sem `Bearer` → 401; token inválido/expirado → 401; `POST /events` desbloqueia (e idempotência); `GET /me/achievements` reflete o estado; id no corpo do evento é ignorado (usa o `sub`)

## Fase 6 — Wire-up, infra local e verificação
- [x] `cmd/server/main.go` — wire-up (config, logger, `telemetry.SetupTracing`, `pgxpool`+`otelpgx`, repo, jwks verifier, service, handler) + cadeia `otelhttp(RouteSpanName(metrics.Wrap(mux)))` + graceful shutdown (HTTP + tracing)
- [x] `services/achievements-service/Dockerfile` — build multi-stage (contexto = raiz; módulo único)
- [x] `infra/docker/docker-compose.yml` — `achievements-postgres` (container próprio, ADR-0015) + `achievements-service` (env `IDENTITY_JWKS_URL`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `depends_on` identity/postgres/tempo); migration+seed via initdb
- [x] `infra/docker/prometheus.yml` — job de scrape do `achievements-service`
- [x] Confirmar `go test ./... -race -cover` e `go build ./...` verdes; todos os Casos de Teste Obrigatórios da spec verdes

## Fase 7 — Collection de teste
- [x] `services/achievements-service/api/achievements.http` — fluxo encadeado: login no identity (`POST /sessions`) → **captura o token** → `POST /events` / `GET /me/achievements` / `GET /achievements` com `Bearer`. Validar o loop: `.http` → trace cruzando identity↔achievements no Grafana/Tempo → linha em `player_achievements`
