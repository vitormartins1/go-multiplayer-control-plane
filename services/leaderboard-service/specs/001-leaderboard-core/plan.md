# Plan: 001-leaderboard-core
**Spec:** specs/001-leaderboard-core/spec.md
**Data:** 2026-06-28

## Abordagem técnica

API REST com `net/http`, hexagonal (ADR-0002). `domain` define `Entry` e as Ports; `service` aplica clamps e orquestra; `storage` é o adapter para **Redis sorted set** (`go-redis/v9`); `transport` traduz HTTP e auth via `shared/auth`. O ranking real-time sai de graça do sorted set: `ZADD GT` (highscore), `ZREVRANGE` (top/around), `ZREVRANK` (posição).

## Storage e compute (ADR-0005, ADR-0015)

**Redis** (sorted set) — segundo desvio do Postgres, ajuste natural para ranking real-time (par AWS = ElastiCache). Local: container Redis no compose. Dados efêmeros no MVP (sem AOF/RDB persistente) — aceitável local; persistência é detalhe de infra de prod.

## Arquivos a criar ou modificar

- `shared/config/config.go` — `RedisAddr` (env `REDIS_ADDR`)
- `services/leaderboard-service/internal/domain/leaderboard.go` — `Entry` + Ports `LeaderboardService`, `LeaderboardStore`, `TokenVerifier`
- `.../internal/domain/errors.go` — `ErrNotRanked`, `ErrInvalidScore`
- `.../internal/service/leaderboard_service.go` (+ test) — clamps, highscore, janela "around"
- `.../internal/storage/redis_store.go` (+ integration test) — `go-redis/v9` (ZADD GT, ZREVRANGE, ZREVRANK)
- `.../internal/transport/http_handler.go` (+ httptest) — `POST /scores`, `GET /leaderboard`, `GET /leaderboard/me`
- `.../cmd/server/main.go`, `.../Dockerfile`, `.../api/leaderboard.http`
- `infra/docker/docker-compose.yml` — `redis` + `redis-exporter` + `leaderboard-service`
- `infra/docker/prometheus.yml` — jobs `leaderboard-service` e `redis`
- `infra/docker/grafana/provisioning/dashboards/redis-overview.json` — dashboard do Redis (ritual de observabilidade)

## Dependências e libs

- `github.com/redis/go-redis/v9` — cliente Redis
- `github.com/testcontainers/testcontainers-go/modules/redis` — Redis real e descartável na integração
- Reaproveita `shared/auth`, `shared/middleware`, `shared/telemetry`

## Decisões técnicas e trade-offs

- **`ZADD GT` para highscore:** `client.ZAddArgs(ctx, key, redis.ZAddArgs{GT: true, Members: ...})` só sobe o score se o novo for maior — a regra de "não rebaixar" mora no Redis, atômica, sem ler-antes-de-escrever.
- **Rank 1-based exposto, 0-based interno:** o `ZREVRANK` do Redis é 0-based; o store devolve isso cru em `Rank(...)` (usado para calcular a janela "around"), e converte para 1-based ao montar `Entry`.
- **`Around`:** `rank0 := store.Rank(playerID)`; `start = max(0, rank0-k)`, `stop = rank0+k`; `store.Range(start, stop)`. `Top(limit)` = `Range(0, limit-1)`.
- **`ErrNotRanked`:** `ZREVRANK`/`ZSCORE` de membro ausente retorna `redis.Nil` → mapeado para `ErrNotRanked` (404).
- **Observabilidade do storage:** `redis-exporter` (oliver006) + job no Prometheus + **dashboard Redis provisionado** — segue o ritual (todo storage entra na stack pelo meio que oferece; aqui, um exporter como o Postgres).
- **Observabilidade de nascença:** herda `shared/` (RED+exemplars, tracing). Sem otelpgx (não há Postgres); o trace cobre o request, e o edge `leaderboard→identity` (JWKS) aparece no service graph.

## Ordem de implementação

1. `shared/config` (REDIS_ADDR)
2. `domain` (Entry, Ports, errors)
3. `service` (clamps, highscore, around) + unit tests (mock store)
4. `storage` (adapter Redis) + integration tests (testcontainers-redis)
5. `transport` (rotas, auth, parse de score/limit/around) + httptest
6. wire-up `main` + Dockerfile + compose (redis + exporter + service) + jobs + dashboard Redis
7. collection `.http`
8. `go test ./... -race` + integração verdes; validar no Docker (submit/top/around com token; highscore; 404)

## Riscos e pontos de atenção

- **API do `go-redis/v9`:** confirmar `ZAddArgs{GT}` e `ZRevRankWithScore` (nomes/assinaturas) — validar na compilação e integração.
- **`redis.Nil` vs erro real:** distinguir "não rankeado" de falha de conexão.
- **testcontainers exige Docker.**
