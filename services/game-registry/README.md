# game-registry

Fonte canônica de **jogos** (ADR-0017). Operacionaliza o multi-tenant por `game_id` (ADR-0015 #4) e acumulou, em torno disso, o cadastro de jogos, o catálogo de modos multiplayer, a presença de jogador por jogo e a Fleet Agones de cada jogo.

- **Protocolo:** REST (`net/http`) · **Porta host:** 8090 (HTTP: API, `/metrics`, `/healthz`)
- **Storage:** Postgres próprio `game_registry` (database-per-service, ADR-0015) · host 5435
- **Dependências:** nenhuma (serviço-folha; é referenciado, não referencia)
- **Auth:** `X-Internal-Key` obrigatória (ADR-0018) nos prefixos `/games` e `/players` — aplicada em `cmd/server/main.go` via `auth.RequireInternalKey`. `/healthz` e `/metrics` ficam livres.

## Responsabilidades

1. **CRUD de jogos** — registro, listagem, consulta, rename, soft delete.
2. **Perfil de multiplayer legado** — alias 1:1 do modo `"default"` (compat pós-ADR-0024).
3. **Modos nomeados (CRUD)** — coleção de modos por jogo (ADR-0024).
4. **Presença de jogador por jogo** — quem já logou em qual jogo (ADR-0019).
5. **Fleet Agones por jogo** — nome da Fleet que aloca game servers do jogo (ADR-0026).
6. **Soft delete** — corte único de visibilidade do jogo em toda a API (ADR-0025).

## API

Todas as rotas abaixo (exceto `/healthz`, `/metrics`) exigem o header `X-Internal-Key` (ADR-0018).

### Jogos (CRUD + soft delete)

| Método | Rota | Descrição |
|---|---|---|
| POST | `/games` | registra um jogo `{slug, name}` -> 201 `{id, slug, name, createdAt, ...}` |
| GET | `/games` | lista jogos (por `name` asc); não inclui perfil/modos |
| GET | `/games/{id}` | consulta um jogo + perfil `default` + resumo dos modos; 404 se ausente/deletado |
| PUT | `/games/{id}` | renomeia (`{name}`); não muda slug nem id |
| PUT | `/games/{id}/slug` | troca o slug (`{slug}`; rename de produto, spec 014) — 400 `invalid_slug`, 409 `slug_taken`; o `id` e a capa gerada não mudam |
| DELETE | `/games/{id}` | soft delete (ADR-0025) -> 204; a partir daí o jogo responde como inexistente em toda a API |
| PUT | `/games/{id}/description` | define descrição curta (`{description}`, até 500 caracteres; vazio limpa) |
| PUT | `/games/{id}/fleet` | define a Fleet Agones (`{fleet}`; vazio limpa) |

`slug` = chave humana (única, case-insensitive, `^[a-z0-9][a-z0-9-]{0,38}[a-z0-9]$`); `id` (UUID) = chave canônica e **estável**, usada pelos outros serviços como `game_id`; o slug pode mudar num rename de produto (spec 014). `fleet` valida o mesmo formato de slug de label Kubernetes (`^[a-z][a-z0-9-]{0,31}$`).

### Perfil de multiplayer — alias legado (spec 006-008)

| Método | Rota | Descrição |
|---|---|---|
| PUT | `/games/{id}/multiplayer-profile` | upsert do perfil do modo `"default"` `{archetype, topology, params}` (ADR-0023) |
| GET | `/games/{id}/multiplayer-profile` | consulta o perfil `"default"`; 404 se o jogo não tiver perfil configurado |

`archetype` ∈ {`queue_arena`, `host_world_dropin`, `battle_royale`, `coop_mission`}; `topology` só suporta `dedicated`. `params` é polimórfico por arquétipo (specs 006/007/008) — chaves fora do vocabulário tipado do arquétipo são preservadas cruas (`ExtraParams`, follow-up #14 do host_world_dropin).

### Modos nomeados (ADR-0024, spec 009)

| Método | Rota | Descrição |
|---|---|---|
| GET | `/games/{id}/multiplayer-profiles` | lista os modos do jogo |
| PUT | `/games/{id}/multiplayer-profiles/{modeId}` | upsert de um modo `{displayName, archetype, topology, params}` (**não** repetir `modeId` no corpo — `DisallowUnknownFields` rejeita) |
| GET | `/games/{id}/multiplayer-profiles/{modeId}` | consulta um modo (inclui a `fleet` do jogo dono, numa única chamada — consumido pelo session-service) |
| DELETE | `/games/{id}/multiplayer-profiles/{modeId}` | remove o modo; o modo `"default"` é protegido (`409 default_mode_protected`) |

`modeId` valida `^[a-z][a-z0-9-]{0,31}$`; limite de 16 modos por jogo (`ErrTooManyModes`). Escritas usam concorrência otimista sobre `multiplayer_profile_version` — uma edição concorrente perde com `409 mode_conflict` em vez de sobrescrever silenciosamente (achado no `/code-review` da spec 009).

### Presença de jogador por jogo (ADR-0019, spec 002/004)

| Método | Rota | Descrição |
|---|---|---|
| PUT | `/games/{gameId}/players/{playerID}` | marca presença (`{handle}`); upsert idempotente, chamado pelo identity no login |
| GET | `/games/{gameId}/players` | lista a base do jogo (ordem de chegada, `first_seen`) |
| GET | `/players/{playerID}/games` | consulta reversa: jogos em que o player está presente |

## Modelo de dados

Tabela única `games`, evoluída por 9 migrations (`migrations/001` a `009`):

| Campo | Migration | Descrição |
|---|---|---|
| `id`, `slug`, `name`, `created_at` | 001 | núcleo do jogo |
| `image`, `image_source`, `generated_image_seed`, `generated_image_style` | 004 | capa gerada no cliente (sem upload no MVP) |
| `multiplayer_profile` (JSONB) | 005 | perfil do modo `"default"` — `{archetype, topology, params}` |
| `deleted_at` | 006 | soft delete (NULL = vivo) |
| `fleet` | 007 | Fleet Agones (`agones.dev/fleet`); NULL/ausente = `""` |
| `multiplayer_profile_version` (BIGINT) | 008 | contador de concorrência otimista da coleção de modos |
| `description` | 009 | texto curto opcional (até 500 caracteres); NULL/ausente = `""` |

Tabela `game_players` (migration 003): presença por jogo — `(game_id, player_id)` PK, `handle` (desnormalizado do login), `first_seen`; índice por `(game_id, first_seen)` para listagem em ordem de chegada.

> A coleção de **modos nomeados** (ADR-0024) não tem tabela própria — hoje é derivada/embutida no mesmo JSONB de perfil por jogo (o `"default"` é o alias legado); ver `internal/domain/game.go` e `internal/service/profile_service.go` para a forma exata.

## Observabilidade

Métricas RED em `/metrics` (`middleware.NewMetrics`) + runtime Go; tracing via `otelhttp` + `otelpgx` (spans de query). Banco com `game-registry-postgres-exporter` (jobs `game-registry` e `game-registry-postgres` no Prometheus). Aparece como nó na topologia do `admin-ui`.

## Testar / Specs

- Manual (HTTP): [`api/game-registry.http`](api/game-registry.http) — **atenção:** o cabeçalho do arquivo ainda diz "sem auth no MVP"; hoje as chamadas exigem `X-Internal-Key`.
- Specs por responsabilidade:
  - Jogos (CRUD/rename/capa/descrição/soft delete): `001-game-core`, `003-generated-cover`, `005-game-rename`, `010-game-soft-delete`, `013-game-description`
  - Presença: `002-game-presence`, `004-games-of-player`
  - Perfil de multiplayer / modos nomeados: `006-multiplayer-profile`, `007-battle-royale-profile`, `008-coop-mission-profile`, `009-multiplayer-modes`, `011-hwd-scoring-stats`
  - Fleet Agones: `012-agones-fleet-field`
