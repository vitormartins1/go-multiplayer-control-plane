# Spec: 002-game-scope

**Serviço:** player-data-service
**Status:** Implementada (2026-06-30)
**Data:** 2026-06-30

## Contexto

Torna o player-data **multi-tenant por jogo** (ADR-0017), como já fizemos em achievements (004) e leaderboard (002). Hoje o dado é por `(player_id, namespace, key)` num Postgres próprio; passa a ser por `(game_id, player_id, namespace, key)`. Player-facing pega o jogo do header `X-Game-Id`; um endpoint **admin read** (internal key, ADR-0018) alimenta a aba **Player data** do painel.

Padrão de referência: `achievements/004-game-scope` (migration com backfill sandbox; PK composta; FK; X-Game-Id) e `achievements/003-admin-read` (endpoint admin sem JWT).

## Comportamento

### Player-facing (X-Game-Id)
- WHEN `PUT/GET/DELETE /me/data/{namespace}/{key}` e `GET /me/data/{namespace}` com header `X-Game-Id=g` -> opera no dado do jogo `g` para o jogador do JWT.
- IF `X-Game-Id` ausente -> `400` (`missing_game`).

### Admin (internal key; jogo/jogador no path)
- WHEN `GET /admin/games/{gameId}/players/{playerID}/data` -> devolve todo o dado do jogador no jogo: lista de `{namespace, key, value}` (para o painel). (Decisão de granularidade no plan — ver Open Questions.)

### Invariante
- THE SYSTEM SHALL escopar todo acesso por `game_id`; dados de jogos diferentes não se misturam.

## Contratos de Interface

- `PlayerDataService` e `PlayerDataRepository` ganham `gameID` em todos os métodos.
- Novo `ListForPlayer(ctx, gameID, playerID) []Entry{Namespace,Key,Value}` (admin).
- Migration `002_add_game_scope.sql`: coluna `game_id TEXT NOT NULL DEFAULT '<sandbox>'`, backfill, nova PK `(game_id, player_id, namespace, key)`, FK opcional (sem FK cross-service — game-registry é outro banco; só a coluna).
- Guard `auth.RequireInternalKey(key, "/admin")` no main; `INTERNAL_API_KEY` no compose.

## Casos de Teste Obrigatórios
- [x] Put/Get isolados por jogo (dado no jogo A não aparece no B)
- [x] player-facing sem `X-Game-Id` -> `400`
- [x] `GET /admin/games/{g}/players/{p}/data` -> lista do jogador no jogo
- [x] `/admin/...` exige internal key
- [x] migration aplica (backfill sandbox; PK nova) — testcontainers

## Open Questions (resolver no plan)
- Admin read: tudo do jogador (`ListForPlayer`) **ou** por namespace (`GET .../data/{namespace}`)? Recomendado: `ListForPlayer` (painel mostra tudo numa tabela; menos cliques).
- Default do `game_id`: usar o id do sandbox (`00000000-0000-0000-0000-000000000000`), igual achievements.

## Fora do escopo
- Consumo no painel (aba Player data) — `admin-bff/006-player-tabs`
- Escrita pelo painel (read-only no admin)
- Quotas/limites por jogo — backlog
