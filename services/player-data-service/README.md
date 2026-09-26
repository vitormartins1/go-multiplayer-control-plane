# player-data-service

≈ **GameKit User Gameplay Data**. Dados de gameplay por jogador em **chave-valor namespaced** (ex. `inventory`, `stats`, `settings`), com `value` em **JSONB**. Persistência server-side = cross-play automático.

- **Protocolo:** REST · **Porta host:** 8083
- **Storage:** PostgreSQL (`player-data-postgres`, DB `player_data`, coluna `value JSONB`)
- **Dependências:** identity (JWKS)

## API (autenticada por Bearer JWT)

| Método | Rota | Descrição |
|---|---|---|
| PUT | `/me/data/{namespace}/{key}` | grava um valor JSON (last-write-wins); até 400 KiB, acima -> 413 `value_too_large` (spec 003) |
| GET | `/me/data/{namespace}/{key}` | lê um valor (404 se ausente) |
| GET | `/me/data/{namespace}` | todos os pares do namespace |
| DELETE | `/me/data/{namespace}/{key}` | remove (idempotente) |

## Modelo de dados

**`player_data`** (PostgreSQL)

| Coluna | Tipo | Notas |
|---|---|---|
| player_id | TEXT | `sub` do JWT |
| namespace | TEXT | agrupador, ex. `inventory` |
| key | TEXT | chave dentro do namespace |
| value | JSONB | valor JSON opaco |
| updated_at | TIMESTAMPTZ | momento da última escrita |
| **PK** | (player_id, namespace, key) | um valor por jogador/namespace/chave |

## Observabilidade

`/metrics` na porta 8083 (**Service Overview**, `$job=player-data-service`). Tracing + otelpgx. Banco no dashboard **PostgreSQL**.

## Testar / Specs

- Manual: [`api/player-data.http`](api/player-data.http)
- Specs: [`specs/`](specs/) — `001-player-data-core`
