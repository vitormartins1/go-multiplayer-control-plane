# game-registry

Fonte canônica de **jogos** (ADR-0017). Operacionaliza o multi-tenant por `game_id` (ADR-0015 #4): registra os jogos que rodam no backend e fornece o `game_id` (UUID) que os demais domínios usam como dimensão. **Control plane interno** — sem auth no MVP.

- **Protocolo:** REST (`net/http`) · **Porta host:** 8090 (HTTP: API, `/metrics`, `/healthz`)
- **Storage:** Postgres próprio `game_registry` (database-per-service, ADR-0015) · host 5435
- **Dependências:** nenhuma (serviço-folha; é referenciado, não referencia)
- **Auth:** nenhuma no MVP (interno/local; provável ADR futuro)

## API

| Método | Rota | Descrição |
|---|---|---|
| POST | `/games` | registra um jogo `{slug, name}` -> 201 `{id, slug, name, createdAt}` |
| GET | `/games` | lista jogos (por `name` asc) |
| GET | `/games/{id}` | consulta um jogo (validação de `game_id`); 404 se ausente |

`slug` = chave humana estável (única, case-insensitive); `id` (UUID) = chave canônica usada pelos outros serviços como `game_id`.

## Modelo de dados

Tabela `games` (`id` TEXT/UUID, `slug` TEXT único via `lower(slug)`, `name`, `created_at`). Migration em `migrations/001_create_games.sql`.

## Observabilidade

Métricas RED em `/metrics` (`middleware.NewMetrics`) + runtime Go; tracing via `otelhttp` + `otelpgx` (spans de query). Banco com `game-registry-postgres-exporter` (jobs `game-registry` e `game-registry-postgres` no Prometheus). Aparece como nó na topologia do `admin-ui`.

## Testar / Specs

- Manual (HTTP): [`api/game-registry.http`](api/game-registry.http)
- Specs: [`specs/`](specs/) — `001-game-core`
