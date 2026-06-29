# Tasks: 001-game-core
**Plan:** specs/001-game-core/plan.md
**Data:** 2026-06-29

> Serviço novo REST + Postgres próprio (ADR-0015/0017). Entra na observabilidade.
> Marcar [x] conforme implementa.

## Fase 1 — Domain
- [ ] `internal/domain/game.go` — `Game{ID,Slug,Name,CreatedAt}`; Ports `GameService` (`Register`/`List`/`Get`) e `GameRepository` (`Create`/`List`/`FindByID`)
- [ ] `internal/domain/errors.go` — `ErrInvalidSlug`, `ErrInvalidName`, `ErrSlugTaken`, `ErrGameNotFound`

## Fase 2 — Migration
- [ ] `migrations/001_create_games.sql` — tabela `games` (id, slug, name, created_at) + índice único em `lower(slug)`

## Fase 3 — Repository
- [ ] `internal/repository/postgres_game.go` — `Create` (`23505` -> `ErrSlugTaken`), `List` (order by `name`), `FindByID` (`ErrGameNotFound`)
- [ ] `internal/repository/postgres_game_test.go` — integration (testcontainers): create/find; slug duplicado case-insensitive -> `ErrSlugTaken`; list por nome; find inexistente -> `ErrGameNotFound`

## Fase 4 — Service
- [ ] `internal/service/game_service.go` — `Register` (valida slug/name, gera uuid/createdAt), `List`, `Get`
- [ ] `internal/service/game_service_test.go` — unit (mock repo): slug inválido; name vazio; slug tomado; register ok; get inexistente

## Fase 5 — Transport
- [ ] `internal/transport/http_handler.go` — `POST /games`, `GET /games`, `GET /games/{id}`; DTO; erros -> 201/400/404/409
- [ ] `internal/transport/http_handler_test.go` — httptest + mock service: 201; 400 (slug/name); 409 (slug tomado); list; get 200/404

## Fase 6 — Wire-up
- [ ] `cmd/server/main.go` — pgxpool + otelpgx; service; transport; middleware RED + RouteSpanName; otelhttp; /healthz; /metrics; graceful shutdown
- [ ] `services/game-registry/Dockerfile` + `api/game-registry.http` + `README.md`

## Fase 7 — Infra e observabilidade
- [ ] `infra/docker/docker-compose.yml` — `game-registry` (8090), `game-registry-postgres` (5435 + migration init script + healthcheck), `game-registry-postgres-exporter`
- [ ] `infra/docker/prometheus.yml` — jobs `game-registry` + `game-registry-postgres`
- [ ] `services/admin-bff/internal/domain/topology.go` — nó `game-registry` em `KnownServices`

## Fase 8 — Verificação
- [ ] `go test ./... -race` + `go test -tags=integration ./services/game-registry/internal/repository/`; `go build ./...`
- [ ] Validar no Docker: registrar/listar/consultar jogo; banco e métricas no Prometheus; nó na topologia do painel
