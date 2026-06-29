# Spec: 001-game-core

**Serviço:** game-registry
**Status:** Approved
**Data:** 2026-06-29

## Contexto

Núcleo do `game-registry` (ADR-0017): a **fonte canônica de jogos**. Operacionaliza o multi-tenant por `game_id` (ADR-0015 #4) — registra os jogos que rodam no backend e fornece o `game_id` que os demais domínios usam como dimensão. É **control plane interno** (consumido pelo painel admin e por serviços que validam o jogo); **sem auth** no MVP, como os demais `/admin`.

No MVP: registrar, listar e consultar um jogo. Sem update/delete, sem configurações de jogo — só a identidade do jogo (`id`, `slug`, `name`).

## Comportamento

### Registrar um jogo
- WHEN `POST /games {slug, name}` válido -> cria o jogo (gera `id` UUID e `createdAt`) e retorna `201 {id, slug, name, createdAt}`.
- IF `slug` inválido -> `400` (**ErrInvalidSlug**).
- IF `name` inválido -> `400` (**ErrInvalidName**).
- IF já existe jogo com o mesmo `slug` (case-insensitive) -> `409` (**ErrSlugTaken**).
- THE SYSTEM SHALL garantir unicidade de `slug` (case-insensitive) — o `slug` é a chave humana estável do jogo.

### Listar jogos
- WHEN `GET /games` -> retorna todos os jogos ordenados por `name` (asc): `[{id, slug, name, createdAt}]`.

### Consultar um jogo (validação por id)
- WHEN `GET /games/{id}` -> retorna o jogo se existir; senão `404` (**ErrGameNotFound**). É o endpoint que outros serviços usam para validar um `game_id`.

## Contratos de Interface

### API HTTP
| Método | Rota | Retorno |
|---|---|---|
| POST | `/games` | `201 {id, slug, name, createdAt}` |
| GET | `/games` | `200 [{id, slug, name, createdAt}]` |
| GET | `/games/{id}` | `200 {id, slug, name, createdAt}` \| `404` |

### GameService (Port de entrada)
- `Register(ctx, slug, name string) (*Game, error)`
- `List(ctx) ([]Game, error)`
- `Get(ctx, id string) (*Game, error)`

### GameRepository (Port de saída)
- `Create(ctx, g *Game) error` — `ErrSlugTaken` em violação de unicidade
- `List(ctx) ([]Game, error)` — por `name` asc
- `FindByID(ctx, id string) (*Game, error)` — `ErrGameNotFound` se ausente

### Tipos (domínio)
- `Game{ ID, Slug, Name string; CreatedAt time.Time }`

## Modelo de Dados

Tabela `games` (Postgres próprio — database-per-service, ADR-0015):

| Coluna | Tipo | Restrição |
|---|---|---|
| `id` | uuid | PK |
| `slug` | text | único (case-insensitive) |
| `name` | text | not null |
| `created_at` | timestamptz | not null |

## Regras de Validação
| Campo | Regra | Erro |
|---|---|---|
| `slug` | 2–40 chars, minúsculas `[a-z0-9-]`, não começa/termina com `-` | `ErrInvalidSlug` |
| `name` | 1–100 chars, não vazio (trim) | `ErrInvalidName` |
| `slug` | único case-insensitive | `ErrSlugTaken` |

## Casos de Teste Obrigatórios

- [ ] `POST /games` válido -> `201` com `id`/`slug`/`name`/`createdAt`
- [ ] `slug` inválido (maiúsculas, curto, char proibido, hífen nas pontas) -> `400 ErrInvalidSlug`
- [ ] `name` vazio/espaços -> `400 ErrInvalidName`
- [ ] `slug` duplicado (case-insensitive: "Tetris" vs "tetris" já normalizado) -> `409 ErrSlugTaken`
- [ ] `GET /games` -> lista os jogos ordenados por `name`
- [ ] `GET /games/{id}` existente -> `200`; inexistente -> `404 ErrGameNotFound`

## Fora do escopo

- **Atualizar / remover** jogo (e soft-delete) — incremental
- **Configurações do jogo** (metadados, settings, chaves) — só identidade no MVP
- **Lookup por `slug`** (`GET /games?slug=`) — `GET /games/{id}` basta no MVP
- **Paginação** — catálogo pequeno; lista simples
- **Auth** — control plane interno/local no MVP; provável ADR futuro
- **Vincular jogos a `game_id` nos outros domínios** — features à parte (achievements primeiro, ADR-0017)
