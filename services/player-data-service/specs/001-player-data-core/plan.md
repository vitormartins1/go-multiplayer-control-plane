# Plan: 001-player-data-core
**Spec:** specs/001-player-data-core/spec.md
**Data:** 2026-06-28

## Abordagem técnica

API REST com `net/http` (ServeMux 1.22, **path params** `{namespace}`/`{key}` via `r.PathValue`), hexagonal (ADR-0002). `domain` define os tipos e Ports; `service` valida e orquestra; `repository` persiste em Postgres (`value` JSONB); `transport` traduz HTTP e aplica o middleware de auth. O valor é **JSON opaco** (`[]byte`) — o serviço não interpreta o conteúdo.

Esta feature **promove o verificador de JWT para `shared/auth`** (2º consumidor do token, como previsto no plan da 001-achievement-core): o `internal/auth` do achievements migra para `shared/auth` (genérico, sem dependência de domain), o achievements passa a usá-lo, e o player-data o consome.

## Compute e banco (ADR-0005, ADR-0015)

- **Banco: PostgreSQL próprio** (`player-data-postgres` / DB `player_data`; par AWS = DynamoDB). `value` como **JSONB**.
- **Local:** container no compose + `player-data-postgres-exporter` (observabilidade por serviço, ritual do OBSERVABILITY.md). Testes de integração com testcontainers.

## shared/auth (extração)

- `shared/auth/jwks_verifier.go` — `Verifier` (cache de JWKS por kid, igual ao atual), com `ErrUnauthorized` **próprio** do pacote (sem importar domain de serviço). `Verify(ctx, token) (playerID string, err error)`.
- Satisfaz **estruturalmente** a Port `TokenVerifier` de qualquer serviço (interfaces de Go são estruturais) — nada a mudar nas Ports.
- **Refactor do achievements:** remove `internal/auth/`; `main.go` passa a usar `shared/auth`; remove o `domain.ErrUnauthorized` órfão (o middleware já mapeia qualquer erro do `Verify` para 401). Testes do verificador migram para `shared/auth`.

## Arquivos a criar ou modificar

- `shared/auth/jwks_verifier.go` + `_test.go` — **novo** (extraído do achievements)
- `services/achievements-service/internal/auth/` — **removido**; `cmd/server/main.go` usa `shared/auth`; `internal/domain/errors.go` e `transport` perdem `ErrUnauthorized`
- `services/player-data-service/internal/domain/player_data.go` — `Entry`/Ports `PlayerDataService`, `PlayerDataRepository`, `TokenVerifier`
- `.../internal/domain/errors.go` — `ErrInvalidKey`, `ErrInvalidValue`, `ErrNotFound`
- `.../internal/service/player_data_service.go` (+ test) — validação de namespace/key, orquestração
- `.../internal/repository/postgres_player_data.go` (+ integration test) — upsert JSONB, get, get-namespace, delete
- `.../migrations/001_create_player_data.sql`
- `.../internal/transport/http_handler.go` (+ httptest) — rotas com path params, auth middleware, `json.Valid`
- `.../cmd/server/main.go`, `.../Dockerfile`, `.../api/player-data.http`
- `infra/docker/docker-compose.yml` — `player-data-service` + `player-data-postgres` + `player-data-postgres-exporter`
- `infra/docker/prometheus.yml` — jobs `player-data-service` e `player-data-postgres`

## Decisões técnicas e trade-offs

- **Path params do Go 1.22:** `mux.HandleFunc("PUT /me/data/{namespace}/{key}", h)` + `r.PathValue("namespace")`. Sem framework; o mux distingue `/me/data/{namespace}` de `/me/data/{namespace}/{key}` pelo número de segmentos.
- **Upsert JSONB:** `INSERT ... ON CONFLICT (player_id, namespace, key) DO UPDATE SET value = EXCLUDED.value, updated_at = EXCLUDED.updated_at`, com `$value::jsonb` (cast explícito — lição GO-LEARNINGS #09).
- **value `[]byte` opaco no domain:** mantém `domain` sem `encoding/json`. A validação "é JSON?" (`json.Valid`) vive no **transport** (formato de entrada); a validação de `namespace`/`key` (regex) vive no **service** (regra de negócio).
- **GET inexistente → `pgx.ErrNoRows` → `domain.ErrNotFound`** (mapeado a 404). DELETE é idempotente (não erra em 0 linhas).
- **Observabilidade de nascença:** herda `shared/` (RED+exemplars, tracing, otelpgx) + exporter do banco.

## Ordem de implementação

1. `shared/auth` (extrai do achievements) + test; refatora o achievements; `go test` do achievements verde
2. `domain` (tipos, Ports, errors)
3. `service` (validação + orquestração) + unit tests
4. `repository` (JSONB upsert/get/delete) + migration + integration tests
5. `transport` (path params, auth, json.Valid) + httptest
6. wire-up `main` + Dockerfile + compose (service + postgres + exporter) + jobs no Prometheus
7. collection `.http`
8. `go test ./... -race` + integração verdes; validar o loop no Docker (PUT/GET/namespace/DELETE com token do identity)

## Riscos e pontos de atenção

- **Refactor do achievements:** garantir que os testes do achievements continuam verdes após trocar `internal/auth` por `shared/auth`.
- **JSONB cast:** `$N::jsonb` para o pgx não tratar o `[]byte` como texto.
- **Boot ordering (JWKS):** o player-data depende do JWKS do identity — fetch lazy + cache (não falha o boot), como no achievements.
- **testcontainers exige Docker.**
