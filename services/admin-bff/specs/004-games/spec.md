# Spec: 004-games

**Serviço:** admin-bff
**Status:** Approved
**Data:** 2026-06-29

## Contexto

Operacionaliza o multi-tenant no painel (ADR-0017): o `admin-bff` ganha a **gestão de jogos** (listar/criar via game-registry) e os **achievements por jogo** (catálogo, criar conquista, achievements de um jogador — via achievements). Também **restaura** a busca de achievements do painel, que ficou quebrada quando o `achievements/004` removeu as rotas admin globais: a versão antiga `GET /api/players/{playerID}/achievements` é **substituída** pela game-scoped.

Read-only continua sendo a regra geral, mas esta feature introduz **dois writes** (criar jogo, criar conquista) — são as ações de setup que o painel precisa. Sem auth no MVP. Lê só as APIs (nunca bancos — ADR-0016).

## Comportamento

### Jogos (via game-registry)
- WHEN `GET /api/games` -> lista os jogos: `[{id, slug, name, createdAt}]`.
- WHEN `POST /api/games {slug, name}` -> cria um jogo; retorna `201 {id, slug, name, createdAt}`.
- IF o game-registry responder erro de validação (`400`/`409`) -> o BFF **repassa** o mesmo status (ex.: slug inválido -> 400; slug já existe -> 409).
- IF o game-registry estiver indisponível -> `502` (**ErrGameRegistryUnavailable**).

### Achievements por jogo (via achievements)
- WHEN `GET /api/games/{gameId}/achievements` -> catálogo de conquistas do jogo: `[{id, name, description, target}]`.
- WHEN `POST /api/games/{gameId}/achievements {id, name, description, eventType, target}` -> cadastra uma conquista no jogo; `201`.
- WHEN `GET /api/games/{gameId}/players/{playerID}/achievements` -> achievements do jogador no jogo (catálogo anotado com progresso).
- IF o achievements responder validação (`400`/`409`) -> o BFF **repassa** o status.
- IF o achievements estiver indisponível -> `502` (**ErrAchievementsUnavailable**).

### Substituição
- A rota `GET /api/players/{playerID}/achievements` (admin-bff/001) é **removida** — não há achievements "sem jogo" no multi-tenant.

## Contratos de Interface

### API HTTP (BFF)
| Método | Rota | Origem |
|---|---|---|
| GET | `/api/games` | game-registry `GET /games` |
| POST | `/api/games` | game-registry `POST /games` |
| GET | `/api/games/{gameId}/achievements` | achievements `GET /admin/games/{gameId}/achievements` |
| POST | `/api/games/{gameId}/achievements` | achievements `POST /admin/games/{gameId}/achievements` |
| GET | `/api/games/{gameId}/players/{playerID}/achievements` | achievements `GET /admin/games/{gameId}/players/{playerID}/achievements` |

### GameRegistry (Port de saída — nova)
- `ListGames(ctx) ([]Game, error)`
- `CreateGame(ctx, slug, name string) (*Game, error)` — repassa erro de validação (UpstreamError com status)

### AchievementsReader (Port de saída — estendida, game-scoped)
- `CatalogForGame(ctx, gameID) ([]Achievement, error)`
- `CreateAchievement(ctx, gameID string, a Achievement) error` — repassa validação
- `PlayerAchievements(ctx, gameID, playerID) ([]PlayerAchievement, error)` (substitui `ForPlayer`)

### Tipos (domínio)
- `Game{ ID, Slug, Name string; CreatedAt time.Time }`
- `Achievement{ ID, Name, Description string; Target int }` (catálogo)
- reuso de `PlayerAchievement`

### Tratamento de erro de upstream
- `UpstreamError{ Status int }` (com `ErrGameRegistryUnavailable`/`ErrAchievementsUnavailable` para indisponibilidade): permite o transport **repassar** 400/409 e mapear indisponibilidade para 502.

## Casos de Teste Obrigatórios

- [ ] `GET /api/games` -> lista do game-registry (fake)
- [ ] `POST /api/games` válido -> 201; slug inválido -> 400; slug duplicado -> 409 (repassados)
- [ ] `GET /api/games/{g}/achievements` -> catálogo do jogo
- [ ] `POST /api/games/{g}/achievements` -> 201; validação -> 400/409 repassados
- [ ] `GET /api/games/{g}/players/{p}/achievements` -> progresso do jogador no jogo
- [ ] game-registry/achievements indisponível -> `502`

## Fora do escopo

- **Atualizar/remover** jogo ou conquista — incremental
- **Auth** — interno/local no MVP
- **Consumo no `admin-ui`** (seção Games) — feature seguinte (admin-ui/003)
- **`game_id` nos demais domínios** — features futuras (ADR-0017)
