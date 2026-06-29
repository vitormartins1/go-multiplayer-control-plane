# Tasks: 001-leaderboard-core
**Plan:** specs/001-leaderboard-core/plan.md
**Data:** 2026-06-28

> Ordem: config → domain → service → storage(Redis) → transport → wire-up/infra → collection.
> Testes acompanham cada camada. Marcar [x] conforme implementa.

## Fase 0 — Base compartilhada
- [x] `shared/config/config.go` — `RedisAddr` (env `REDIS_ADDR`)

## Fase 1 — Domain
- [x] `internal/domain/leaderboard.go` — `Entry{Rank, PlayerID, Score}` + Ports `LeaderboardService`, `LeaderboardStore`, `TokenVerifier`
- [x] `internal/domain/errors.go` — `ErrNotRanked`, `ErrInvalidScore`

## Fase 2 — Service
- [x] `internal/service/leaderboard_service.go` — `Submit` (highscore + retorna entry), `Top` (clamp limit), `Around` (clamp k, janela rank±k)
- [x] `internal/service/leaderboard_service_test.go` — unit (mock store): submit→entry; top com clamp; around (start/stop calculados); not-ranked propaga; clamps de limit/k

## Fase 3 — Storage (adapter Redis)
- [x] `internal/storage/redis_store.go` — `go-redis/v9`: `Submit` (ZADD GT), `Rank` (ZREVRANK+score, `redis.Nil`→`ErrNotRanked`), `Top`/`Range` (ZREVRANGE WITHSCORES, rank 1-based)
- [x] `internal/storage/redis_store_test.go` — integration (testcontainers-redis): submit/top; highscore (menor não rebaixa); rank de inexistente→ErrNotRanked; range/around

## Fase 4 — Transport
- [x] `internal/transport/http_handler.go` — middleware auth (`shared/auth`); `POST /scores`, `GET /leaderboard?limit=`, `GET /leaderboard/me?around=`; parse/erros→HTTP (401/400/404)
- [x] `internal/transport/http_handler_test.go` — httptest: submit→top; highscore; me sem score→404; score inválido→400; sem token→401

## Fase 5 — Wire-up, infra e verificação
- [x] `cmd/server/main.go` — wire-up (config, logger, tracing, redis store, service, handler, shared/auth) + cadeia observável + graceful shutdown
- [x] `services/leaderboard-service/Dockerfile`
- [x] `infra/docker/docker-compose.yml` — `redis` + `redis-exporter` + `leaderboard-service` (env REDIS_ADDR/JWKS/OTEL, depends_on)
- [x] `infra/docker/prometheus.yml` — jobs `leaderboard-service` e `redis`
- [x] `infra/docker/grafana/provisioning/dashboards/redis-overview.json` — dashboard do Redis
- [x] `go test ./... -race -cover` + `-tags=integration` verdes; `go build ./...`
- [x] Validar no Docker: submit/top/around com token; highscore; 404; métricas do Redis no Prometheus

## Fase 6 — Collection
- [x] `services/leaderboard-service/api/leaderboard.http` — login no identity → token → POST /scores, GET /leaderboard, GET /leaderboard/me
