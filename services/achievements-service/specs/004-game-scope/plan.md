# Plan: 004-game-scope
**Spec:** specs/004-game-scope/spec.md
**Data:** 2026-06-29

## Abordagem técnica

Atravessa as 4 camadas + migração. `game_id` vira parâmetro em todas as Ports (service e repository) e coluna nas duas tabelas. No transport, o `game_id` é lido do header **`X-Game-Id`** nas rotas de jogador e do **path** `{gameId}` nas rotas admin. Adiciona o `CreateAchievement` (catálogo gerenciável pelo painel). Migração com **jogo `sandbox`** (id fixo) seedado também no game-registry. Sem FK cross-service nem validação do jogo em runtime (ADR-0015/0017).

**Constante do jogo default:** `sandbox` = id `00000000-0000-0000-0000-000000000000` (UUID nil, sentinela legível). Usada no seed do game-registry e no backfill do achievements (precisam casar).

## Arquivos a criar ou modificar

**Migração (achievements DB)**
- `migrations/003_add_game_scope.sql` —
  - `achievements`: `ADD COLUMN game_id TEXT NOT NULL DEFAULT '<sandbox>'`; `DROP CONSTRAINT achievements_pkey`; `ADD PRIMARY KEY (game_id, id)`
  - `player_achievements`: `ADD COLUMN game_id TEXT NOT NULL DEFAULT '<sandbox>'`; `DROP CONSTRAINT player_achievements_achievement_id_fkey`; `DROP CONSTRAINT player_achievements_pkey`; `ADD PRIMARY KEY (game_id, player_id, achievement_id)`; `ADD FOREIGN KEY (game_id, achievement_id) REFERENCES achievements (game_id, id)`
  - após o backfill: `ALTER COLUMN game_id DROP DEFAULT` nas duas (força `game_id` explícito em inserts novos — pega bug de escopo)

**Companion (game-registry)**
- `services/game-registry/migrations/002_seed_sandbox.sql` — `INSERT INTO games (id, slug, name, created_at) VALUES ('<sandbox>', 'sandbox', 'Sandbox', now()) ON CONFLICT DO NOTHING`
- `infra/docker/docker-compose.yml` — montar a 002 no `game-registry-postgres` (init script)

**Domain**
- `internal/domain/achievement.go` — `Achievement` += `GameID`; **todas** as assinaturas de `AchievementService` e `AchievementRepository` ganham `gameID string`; novo `CreateAchievement(ctx, gameID, a)` nas duas Ports
- `internal/domain/errors.go` — `ErrAchievementExists`, `ErrInvalidAchievement`

**Service**
- `internal/service/achievement_service.go` — repassar `gameID` em `ListCatalog`/`ListForPlayer`/`RecordEvent`; novo `CreateAchievement` (valida slug `[a-z0-9-]` 2–40, name 1–100, target>=1; chama `repo.CreateAchievement`)
- `internal/service/achievement_service_test.go` — atualizar todas as chamadas com `gameID`; testes do `CreateAchievement` (inválido; ok; já existe)

**Repository**
- `internal/repository/postgres_achievement.go` — `game_id` em todas as queries (WHERE/INSERT); `RecordProgress` muda o `ON CONFLICT` para `(game_id, player_id, achievement_id)` e o INSERT inclui `game_id`; novo `CreateAchievement` (`INSERT ... ; 23505 -> ErrAchievementExists`)
- `internal/repository/postgres_achievement_test.go` — integration: `game_id` em todos; **teste de isolamento** (evento/catálogo de A não vaza para B); mesmo `id` em jogos diferentes coexiste

**Transport**
- `internal/transport/http_handler.go` —
  - helper `gameID(r)` lê `X-Game-Id`; vazio -> `400 missing_game`
  - rotas de jogador (`/events`, `/me/achievements`, `/achievements`): passam o `gameID` do header
  - **substitui** as rotas admin da 003 por: `POST /admin/games/{gameId}/achievements` (create), `GET /admin/games/{gameId}/achievements` (list), `GET /admin/games/{gameId}/players/{playerID}/achievements`
- `internal/transport/http_handler_test.go` — header `X-Game-Id` nos testes de jogador; novas rotas admin; testes do create (201/400/409)

**Docs**
- `api/achievements.http` — header `X-Game-Id` nos exemplos de jogador; novas rotas admin por jogo

> `cmd/server/main.go` **sem mudança** (assinatura de `service.New`/`NewHandler` inalterada).

## Decisões técnicas e trade-offs
- **PK `(game_id, id)` + FK composta**: o slug do achievement é único **por jogo**; a FK de progresso aponta para `(game_id, id)`. É a modelagem canônica de multi-tenant por linha.
- **`DROP DEFAULT` pós-backfill**: o default `sandbox` serve só pra migrar os dados existentes; depois, todo insert precisa de `game_id` explícito — qualquer caminho que "esqueça" o jogo falha cedo (não grava silenciosamente no sandbox).
- **Header `X-Game-Id` (data plane) vs path (admin)**: contexto ambiente no data plane (SDK injeta), recurso no path no admin — padrão de game BaaS (ver spec).
- **`sandbox` seedado de verdade no game-registry**: o `game_id` default aponta para um jogo que existe (consistência por id+API, ADR-0015), em vez de um sentinela solto.
- **Sem validação do jogo em runtime**: achievements confia no `game_id` do caller (desacopla de game-registry); validação é evolução.

## Ordem de implementação
1. game-registry: seed `sandbox` (002) + compose
2. achievements: migração 003
3. domain (Achievement+GameID, Ports +gameID, CreateAchievement, erros)
4. repository (+game_id, CreateAchievement) + integration tests (isolamento)
5. service (+gameID, CreateAchievement, validação) + unit tests
6. transport (header + rotas admin por jogo + create) + tests
7. api/.http
8. `go test ./... -race` + integration; `go build ./...`; Docker (`down -v` para aplicar a migração): isolamento entre jogos, create via rota admin, eventos por jogo

## Riscos e pontos de atenção
- **Quebra de contrato admin (003) -> admin-bff:** o adapter de achievements do `admin-bff/001` chama `GET /admin/players/{id}/achievements` (rota antiga, agora removida). Depois desta feature, a busca de achievements no painel **404/502** até o `admin-bff/004-games` + `admin-ui/003-games` passarem o `gameId`. **Sequenciar essas duas logo em seguida.** (Não quebra build; é regressão funcional do painel.)
- **Migração só roda em volume novo:** init script aplica a 003 **só na criação** do volume do `achievements-postgres`. Em dev com volume existente, fazer `docker compose down -v` (reseta os bancos) para aplicar. Documentar.
- **Nomes de constraint do Postgres:** `achievements_pkey`, `player_achievements_pkey`, `player_achievements_achievement_id_fkey` (padrão). Validar no integration test (a migração roda lá via init script fresco).
- **Churn de testes:** 3 arquivos de teste (service/transport/repository) ganham `gameID` — atualizar todas as chamadas.
- **Constante `sandbox` precisa casar** entre o seed do game-registry e o backfill do achievements (mesmo id).
