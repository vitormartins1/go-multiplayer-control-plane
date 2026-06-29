# Tasks: 004-games
**Plan:** specs/004-games/plan.md
**Data:** 2026-06-29

> Gestão de jogos (game-registry) + achievements por jogo (achievements). Writes
> repassam 4xx via UpstreamError. Marcar [x] conforme implementa.

## Fase 1 — Domain
- [x] `internal/domain/admin.go` — tipos `Game`, `Achievement` (catálogo); Port `GameRegistry` (`ListGames`, `CreateGame`); estende `AchievementsReader` (`CatalogForGame`, `CreateAchievement`, `PlayerAchievements`; remove `ForPlayer`)
- [x] `internal/domain/errors.go` — `ErrGameRegistryUnavailable`; tipo `UpstreamError{Status}`

## Fase 2 — Game-registry client (adapter)
- [x] `internal/gameregistry/client.go` — `ListGames`/`CreateGame`; helper de classificação (4xx -> UpstreamError; conn/5xx -> ErrGameRegistryUnavailable)
- [x] `internal/gameregistry/client_test.go` — `httptest`: list; create 201; 409 -> UpstreamError(409); fora -> unavailable

## Fase 3 — Achievements client (game-scoped)
- [x] `internal/achievements/client.go` — trocar `ForPlayer` por `CatalogForGame`, `CreateAchievement`, `PlayerAchievements` (rotas `/admin/games/{g}/...`); mesmo tratamento de erro
- [x] `internal/achievements/client_test.go` — atualizar; create 201/409; catálogo; player achievements

## Fase 4 — Service
- [x] `internal/service/admin_service.go` — `New` recebe `GameRegistry`; `Games`/`CreateGame`/`GameCatalog`/`CreateAchievement`/`PlayerAchievements(gameID, playerID)` (remove o antigo)
- [x] `internal/service/admin_service_test.go` — `New(...)` + fakes; novos testes

## Fase 5 — Transport
- [x] `internal/transport/http_handler.go` — rotas `/api/games`, `/api/games/{gameId}/achievements`, `/api/games/{gameId}/players/{playerID}/achievements`; remove `GET /api/players/{playerID}/achievements`; helper repassa `UpstreamError.Status`, indisponível -> 502
- [x] `internal/transport/http_handler_test.go` — `fakeService` + testes (200/201/400/409/502)

## Fase 6 — Wire-up, infra e verificação
- [x] `cmd/server/main.go` — `GAME_REGISTRY_URL` + client gameregistry (otelhttp); injeta no `service.New`
- [x] `api/README.md` + `README.md` — rotas de jogos/achievements por jogo
- [x] `infra/docker/docker-compose.yml` — `GAME_REGISTRY_URL` + `depends_on: game-registry`
- [x] `go test ./... -race`; `go build ./...`; validar no Docker (listar/criar jogo; cadastrar/listar achievements por jogo; repasse de 409)
