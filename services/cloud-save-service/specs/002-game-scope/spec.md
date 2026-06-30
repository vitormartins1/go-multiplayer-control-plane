# Spec: 002-game-scope

**Serviço:** cloud-save-service
**Status:** Implementada (2026-06-30)
**Data:** 2026-06-30

## Contexto

Torna o cloud-save **multi-tenant por jogo** (ADR-0017). Hoje o save é um objeto S3/MinIO com key = `playerID` (um save por jogador, global). Passa a ser **por jogo**: key = `{gameID}/{playerID}`. Player-facing pega o jogo do header `X-Game-Id`; um endpoint **admin read** (internal key, ADR-0018) alimenta a aba **Cloud saves** do painel.

Padrão de referência: `leaderboard/002-game-scope` (key por jogo no store) e `achievements/003-admin-read` (admin sem JWT).

## Comportamento

### Player-facing (X-Game-Id)
- WHEN `PUT /me/save` / `GET /me/save` com header `X-Game-Id=g` -> grava/lê o save do jogo `g` para o jogador do JWT (sobrescreve no PUT).
- IF `X-Game-Id` ausente -> `400` (`missing_game`).

### Admin (internal key; jogo/jogador no path)
- WHEN `GET /admin/games/{gameId}/players/{playerID}/save` -> devolve o save do jogador no jogo (conteúdo + metadados: tamanho, updatedAt). `404` se não existir.

### Invariante
- THE SYSTEM SHALL escopar todo acesso por `game_id` (prefixo da key S3); saves de jogos diferentes não se misturam.

## Contratos de Interface

- `CloudSaveService` e `SaveStore` ganham `gameID` nos métodos (`Save`/`Load`/`Put`/`Get`).
- S3 key: `objectKey(gameID, playerID) = gameID + "/" + playerID`.
- Novo método admin no service: `LoadFor(ctx, gameID, playerID) (Snapshot, error)` (reusa `Get`).
- Guard `auth.RequireInternalKey(key, "/admin")` no main; `INTERNAL_API_KEY` no compose.

## Casos de Teste Obrigatórios
- [x] Save/Load isolados por jogo (save no jogo A não aparece no B)
- [x] player-facing sem `X-Game-Id` -> `400`
- [x] `GET /admin/games/{g}/players/{p}/save` -> save do jogador; `404` se ausente
- [x] `/admin/...` exige internal key

## Open Questions (resolver no plan)
- Admin read devolve o **conteúdo** do snapshot ou só **metadados** (tamanho/updatedAt)? Recomendado: metadados + conteúdo (saves são pequenos no MVP); o painel mostra metadados e um preview/download.
- Migração de objetos globais antigos: não migra (S3/MinIO dev recomeça por jogo) — documentar.

## Fora do escopo
- Consumo no painel (aba Cloud saves) — `admin-bff/006-player-tabs`
- Versionamento/histórico de saves — backlog
- Escrita pelo painel (read-only no admin)
