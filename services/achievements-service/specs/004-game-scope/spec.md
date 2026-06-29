# Spec: 004-game-scope

**Serviço:** achievements-service
**Status:** Approved
**Data:** 2026-06-29

## Contexto

Primeira ponta do multi-tenant por jogo (ADR-0017): o achievements ganha a dimensão **`game_id`**. O catálogo de conquistas e o progresso dos jogadores passam a ser **por jogo**, e o painel admin pode **cadastrar o catálogo de um jogo** (registrar jogo → cadastrar achievements).

Decisões de mecanismo (alinhadas ao padrão de game BaaS — ver discussão / ADR-0017):
- **Data plane** (chamadas do game client / game server): o jogo é **contexto ambiente** via header **`X-Game-Id`** (identidade é global, ADR-0009, então o jogo **não** vem do token). `playerID` segue do JWT, `game_id` do header.
- **Admin** (painel): o jogo é **recurso no path** — `/admin/games/{gameId}/...`.
- **Jogo default:** a migração cria/usa um jogo **`sandbox`** (id fixo bem-conhecido), seedado também no game-registry, e atribui o catálogo + progresso existentes a ele (preserva os dados, ADR-0017).
- **Sem validação** do `game_id` contra o game-registry em runtime no MVP (achievements confia no caller; validação é evolução).

## Comportamento

### Data plane (client / game server) — escopo por header
- WHEN `POST /events` com header `X-Game-Id=g` (e JWT do jogador `p`) -> avalia **apenas** as conquistas do jogo `g`; o progresso é gravado por **(g, p)**; retorna as recém-desbloqueadas em `g`.
- WHEN `GET /me/achievements` com `X-Game-Id=g` -> retorna o catálogo de `g` anotado com o progresso de `p` **em `g`**.
- WHEN `GET /achievements` com `X-Game-Id=g` -> retorna o catálogo do jogo `g`.
- IF o header `X-Game-Id` estiver ausente ou vazio nessas rotas -> `400` (**ErrMissingGame**).
- THE SYSTEM SHALL escopar toda leitura/escrita de catálogo e progresso por `game_id`: dados de um jogo nunca aparecem em outro.

### Admin (painel) — escopo no path
- WHEN `POST /admin/games/{gameId}/achievements {id, name, description, eventType, target}` -> cria uma conquista no catálogo do jogo `{gameId}` e retorna `201`.
- IF `id` (slug) / `name` inválido -> `400`; IF já existe conquista com esse `id` **no mesmo jogo** -> `409` (**ErrAchievementExists**). O mesmo `id` em **jogos diferentes** é permitido.
- WHEN `GET /admin/games/{gameId}/achievements` -> lista o catálogo do jogo `{gameId}`.
- WHEN `GET /admin/games/{gameId}/players/{playerID}/achievements` -> o catálogo de `{gameId}` anotado com o progresso de `{playerID}` nesse jogo.
- THE SYSTEM SHALL manter os endpoints `/admin/...` sem auth no MVP (interno).

> As rotas admin globais da 003 (`GET /admin/achievements`, `GET /admin/players/{id}/achievements`) são **substituídas** pelas versões com `{gameId}` — não há catálogo "global" no mundo multi-tenant.

## Contratos de Interface

### API HTTP
| Método | Rota | Escopo do jogo |
|---|---|---|
| POST | `/events` | header `X-Game-Id` |
| GET | `/me/achievements` | header `X-Game-Id` |
| GET | `/achievements` | header `X-Game-Id` |
| POST | `/admin/games/{gameId}/achievements` | path |
| GET | `/admin/games/{gameId}/achievements` | path |
| GET | `/admin/games/{gameId}/players/{playerID}/achievements` | path |

### AchievementService (Port de entrada) — todos ganham `gameID`
- `ListCatalog(ctx, gameID) ([]Achievement, error)`
- `ListForPlayer(ctx, gameID, playerID) ([]PlayerAchievement, error)`
- `RecordEvent(ctx, gameID, playerID, event) ([]Achievement, error)`
- `CreateAchievement(ctx, gameID string, a Achievement) (*Achievement, error)` — **novo**; `ErrAchievementExists` se o id já existir no jogo

### AchievementRepository (Port de saída) — todos ganham `gameID`
- `ListCatalog(ctx, gameID)`; `FindByEventType(ctx, gameID, eventType)`; `ListPlayerProgress(ctx, gameID, playerID)`; `RecordProgress(ctx, gameID, playerID, achievementID, target, at)`; `CreateAchievement(ctx, gameID, *Achievement)` (**novo**, `23505` -> `ErrAchievementExists`)

### Tipos (domínio)
- `Achievement` ganha `GameID string` (o catálogo é por jogo).

## Modelo de Dados (migração 003 — achievements DB)

- `achievements`: + coluna `game_id` (TEXT, not null). **PK passa a ser `(game_id, id)`** — o `id` (slug) é único **por jogo**.
- `player_achievements`: + coluna `game_id` (TEXT, not null); **PK `(game_id, player_id, achievement_id)`**; FK -> `achievements (game_id, id)`.
- **Backfill:** registros existentes recebem `game_id = <sandbox>` (jogo default). O seed da 001/002 fica sob o jogo `sandbox`.
- **Companion (game-registry):** seed do jogo `sandbox` (id fixo) para o `game_id` default apontar para um jogo real.

`game_id` é **TEXT, sem FK cross-service** (ADR-0015): achievements não faz JOIN em `games`.

## Regras de Validação
| Campo | Regra | Erro |
|---|---|---|
| `X-Game-Id` (data plane) | presente, não vazio | `ErrMissingGame` (400) |
| `id` (slug da conquista) | 2–40, `[a-z0-9-]` minúsculo, sem hífen nas pontas | `400` |
| `name` | 1–100, não vazio | `400` |
| `target` | inteiro >= 1 (default 1) | `400` |
| `id` no jogo | único por `game_id` | `ErrAchievementExists` (409) |

## Casos de Teste Obrigatórios

- [ ] `POST /events` (game A) só progride/desbloqueia conquistas de A; o mesmo player em B fica intacto (isolamento por jogo)
- [ ] `GET /me/achievements` (X-Game-Id=A) retorna só o catálogo+progresso de A
- [ ] data plane sem `X-Game-Id` -> `400 ErrMissingGame`
- [ ] `POST /admin/games/{g}/achievements` cria no catálogo de `g`; id duplicado **no mesmo jogo** -> `409`; **mesmo id em outro jogo** -> ok
- [ ] `GET /admin/games/{g}/achievements` lista só o catálogo de `g`
- [ ] `GET /admin/games/{g}/players/{p}/achievements` -> progresso de `p` em `g`
- [ ] (integration) migração: dados existentes migram para o jogo `sandbox`; `(game_id, id)` PK; isolamento entre jogos

## Fora do escopo

- **Validação do `game_id`** contra o game-registry em runtime — evolução
- **Update / delete** de conquista — incremental
- **Consumo no `admin-bff` / `admin-ui`** (catálogo por jogo no painel) — features à parte (próximas)
- **`game_id` nos demais domínios** (leaderboard, player-data, cloud-save) — features futuras (ADR-0017)
- **Visão "quais jogos um player joga"** (cross-game) — backlog
