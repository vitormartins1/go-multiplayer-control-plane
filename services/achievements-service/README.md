# achievements-service

≈ **GameKit Achievements**. Catálogo de conquistas (por seed), **desbloqueio por evento de gameplay** (idempotente) e **progresso parcial** (contadores até a meta). Primeiro serviço a validar o JWT do identity (via `shared/auth`).

- **Protocolo:** REST · **Porta host:** 8082
- **Storage:** PostgreSQL (`achievements-postgres`, DB `achievements`)
- **Dependências:** identity (JWKS)

## API (autenticada por Bearer JWT)

| Método | Rota | Descrição |
|---|---|---|
| POST | `/events` | registra um evento (`type`) → desbloqueia/progride; retorna as recém-desbloqueadas |
| GET | `/achievements` | catálogo |
| GET | `/me/achievements` | conquistas do jogador (progresso + desbloqueadas) |

## Modelo de dados

**`achievements`** (catálogo, definido por seed)

| Coluna | Tipo | Notas |
|---|---|---|
| id | TEXT | PK (slug, ex. `first_win`) |
| name / description | TEXT | exibíveis |
| event_type | TEXT | tipo de evento que dispara |
| target | INT | meta (default 1; `1` = desbloqueio direto) |

**`player_achievements`** (progresso/desbloqueio do jogador)

| Coluna | Tipo | Notas |
|---|---|---|
| player_id | TEXT | `sub` do JWT (UUID ou `guest-…`) |
| achievement_id | TEXT | FK → `achievements.id` |
| progress | INT | ocorrências acumuladas |
| unlocked_at | TIMESTAMPTZ (nullable) | `NULL` = em progresso |
| **PK** | (player_id, achievement_id) | garante a idempotência do desbloqueio |

## Observabilidade

`/metrics` na porta 8082 (dashboard **Service Overview**, `$job=achievements-service`). Tracing no Tempo (inclui spans de query do Postgres via otelpgx). Banco no dashboard **PostgreSQL**.

## Testar / Specs

- Manual: [`api/achievements.http`](api/achievements.http)
- Specs: [`specs/`](specs/) — `001-achievement-core`, `002-partial-progress`
