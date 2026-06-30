# Plan: 002-game-scope
**Spec:** specs/002-game-scope/spec.md
**Data:** 2026-06-30

## Abordagem técnica
`gameID` entra em todos os métodos de `LeaderboardStore`/`LeaderboardService`.
O Redis usa **um sorted set por jogo**: `leaderboard:{gameID}` (a key fixa vira
prefixo + gameID). Player-facing pega o jogo do header `X-Game-Id`; admin pega
do path e exige internal key (guard no main, ADR-0018).

## Arquivos
- `internal/domain/leaderboard.go` — `gameID` em Submit/Top/Around (service) e Submit/Rank/Top/Range (store)
- `internal/storage/redis_store.go` — key por jogo (`prefix:gameID`); métodos recebem gameID
- `internal/storage/redis_store_test.go` — isolamento por jogo (testcontainers)
- `internal/service/leaderboard_service.go` — repassa gameID; `Top(ctx, gameID, limit)` (admin)
- `internal/service/leaderboard_service_test.go` — gameID propagado; isolamento
- `internal/transport/http_handler.go` — player: `gameFromHeader` (X-Game-Id, vazio->400); admin: `POST /admin/games/{gameId}/players/{playerID}/scores`, `GET /admin/games/{gameId}/leaderboard`
- `internal/transport/http_handler_test.go` — X-Game-Id; rotas admin
- `cmd/server/main.go` — aplica `auth.RequireInternalKey(key, "/admin")`; store com prefixo de key
- `infra/docker/docker-compose.yml` — leaderboard: `INTERNAL_API_KEY`
- `api/leaderboard.http` — X-Game-Id nos player + exemplos admin (X-Internal-Key)

## Decisões
- **Sorted set por jogo** (`leaderboard:{gameID}`): isolamento natural no Redis; sem migração do global (dev recomeça).
- **X-Game-Id (player) / path (admin)**: mesmo padrão do achievements (ADR-0017).
- **Top admin** reusa `store.Top` (que já existe) com a key do jogo.

## Ordem
1. domain (assinaturas) -> storage (key por jogo) + test -> service + test
2. transport (X-Game-Id + rotas admin) + test
3. main (guard internal key + key prefix) + compose + .http
4. `go test -race` (testcontainers) + build; validar no Docker (isolamento + admin)

## Riscos
- **testcontainers Redis** no storage_test: já existe no 001; só estender com gameID.
- **Guard /admin**: lembrar de aplicar no main (leaderboard ainda não tinha /admin).
- **Player sem X-Game-Id**: 400 claro (não cair em key vazia).
