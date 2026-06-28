# Tasks: 001-player-data-core
**Plan:** specs/001-player-data-core/plan.md
**Data:** 2026-06-28

> Ordem: shared/auth → domain → service → repository → transport → wire-up/infra → collection.
> Testes acompanham cada camada. Marcar [x] conforme implementa.

## Fase 0 — shared/auth (extração; 2º consumidor do JWT)
- [x] `shared/auth/jwks_verifier.go` — mover o `Verifier` do achievements; `ErrUnauthorized` próprio do pacote (sem importar domain de serviço)
- [x] `shared/auth/jwks_verifier_test.go` — migrar os testes do verificador
- [x] Refatorar achievements: remover `internal/auth/`; `cmd/server/main.go` usa `shared/auth`; remover `domain.ErrUnauthorized` órfão e o case no `writeDomainError`
- [x] `go test ./services/achievements-service/... -race` verde após o refactor

## Fase 1 — Domain
- [x] `internal/domain/player_data.go` — Ports `PlayerDataService`, `PlayerDataRepository` (value `[]byte` opaco), `TokenVerifier`
- [x] `internal/domain/errors.go` — `ErrInvalidKey`, `ErrInvalidValue`, `ErrNotFound`

## Fase 2 — Service
- [x] `internal/service/player_data_service.go` — `Get`, `GetNamespace`, `Put`, `Delete`; valida `namespace`/`key` (regex) → `ErrInvalidKey`
- [x] `internal/service/player_data_service_test.go` — unit (mock repo): put→get; sobrescrita; get inexistente→ErrNotFound; namespace/key inválido→ErrInvalidKey; isolamento por jogador

## Fase 3 — Repository
- [x] `migrations/001_create_player_data.sql` — tabela `player_data` (PK `(player_id, namespace, key)`, `value JSONB`, `updated_at`)
- [x] `internal/repository/postgres_player_data.go` — `Put` (upsert JSONB com `$value::jsonb`), `Get`, `GetNamespace`, `Delete`; `pgx.ErrNoRows` → `ErrNotFound`
- [x] `internal/repository/postgres_player_data_test.go` — integration (testcontainers): put/get; sobrescrita; get inexistente; get-namespace; delete idempotente; isolamento por jogador

## Fase 4 — Transport
- [x] `internal/transport/http_handler.go` — middleware de auth (`shared/auth`); rotas `PUT/GET/DELETE /me/data/{namespace}/{key}` e `GET /me/data/{namespace}` (PathValue 1.22); `json.Valid` no corpo do PUT; erros→HTTP (401/400/404/204)
- [x] `internal/transport/http_handler_test.go` — httptest: PUT→GET; 404; namespace; DELETE idempotente; key inválida→400; corpo não-JSON→400; sem token→401; isolamento

## Fase 5 — Wire-up, infra e verificação
- [x] `cmd/server/main.go` — wire-up (config, logger, tracing, pgxpool+otelpgx, shared/auth verifier, repo, service, handler) + cadeia observável + graceful shutdown
- [x] `services/player-data-service/Dockerfile`
- [x] `infra/docker/docker-compose.yml` — `player-data-postgres` + `player-data-postgres-exporter` + `player-data-service` (env JWKS/OTEL, depends_on)
- [x] `infra/docker/prometheus.yml` — jobs `player-data-service` e `player-data-postgres`
- [x] `go test ./... -race -cover` + `-tags=integration` verdes; `go build ./...`
- [x] Validar no Docker: PUT/GET/namespace/DELETE com token do identity; trace com spans JSONB; banco no dashboard PostgreSQL

## Fase 6 — Collection
- [x] `services/player-data-service/api/player-data.http` — login no identity → token → PUT/GET/namespace/DELETE
