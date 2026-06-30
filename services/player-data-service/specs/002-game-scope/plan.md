# Plan: 002-game-scope
**Spec:** specs/002-game-scope/spec.md
**Data:** 2026-06-30

## Abordagem técnica
`game_id` entra na PK e em todos os métodos (domain/service/repository/transport),
espelhando `achievements/004-game-scope`. Player-facing pega do `X-Game-Id`; admin
do path + internal key. Admin read = `ListForPlayer` (tudo do jogador no jogo).

## Arquivos
- `migrations/002_add_game_scope.sql` — `ALTER TABLE player_data ADD COLUMN game_id TEXT NOT NULL DEFAULT '00000000-0000-0000-0000-000000000000'`; recriar PK `(game_id, player_id, namespace, key)` (DROP/ADD PK); índice por `(game_id, player_id, namespace)`
- `internal/domain/player_data.go` — `gameID` em Service/Repository; novo `ListForPlayer`; tipo `Entry{Namespace,Key,Value}`
- `internal/repository/postgres_player_data.go` — todas as queries com `game_id = $1`; `ListForPlayer` (`SELECT namespace,key,value WHERE game_id=$1 AND player_id=$2`)
- `internal/service/player_data_service.go` — repassa gameID; `ListForPlayer`
- `internal/transport/http_handler.go` — player: `gameFromHeader` (X-Game-Id, vazio->400); admin: `GET /admin/games/{gameId}/players/{playerID}/data`
- `cmd/server/main.go` — `auth.RequireInternalKey(key, "/admin")`; lê `INTERNAL_API_KEY`
- `infra/docker/docker-compose.yml` — `INTERNAL_API_KEY` + montar a migration 002 no volume do `player-data-postgres` (init só roda em volume novo -> `down -v`)
- testes: unit (service/transport) + integração (repository, testcontainers) com isolamento por jogo
- `api/*.http` — `X-Game-Id` nos player + exemplo admin (X-Internal-Key)

## Decisões
- **Migration**: cuidado com a ordem PK (DROP antiga -> ADD nova); backfill via DEFAULT já preenche linhas existentes.
- **Sem FK cross-service**: game-registry é outro banco (ADR-0015); só a coluna `game_id`.
- **Admin read = ListForPlayer** (tudo do jogador no jogo) — melhor para o painel.

## Ordem
1. migration + domain
2. repository (+ integração) -> service (+ unit) -> transport (+ unit)
3. main (guard) + compose (env + volume + `down -v`)
4. `go test -race` + build; Docker: isolamento por jogo + admin com/sem key

## Riscos
- **Migration em volume existente** não roda: precisa `docker compose down -v` do player-data-postgres (lembrar no checklist).
- **Churn de testes** (assinaturas com gameID) — igual aos serviços anteriores.
