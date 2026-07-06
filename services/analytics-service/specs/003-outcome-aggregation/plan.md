# Plan: 003-outcome-aggregation

**Spec:** `spec.md` (Aprovada) · **ADR:** 0023

## Decisões técnicas

### D1 — Domínio
- `PlayerResult` ganha `Outcome string` (`"win"|"loss"|"draw"`). `Won bool` **permanece** (fallback/compat), mas a agregação passa a discriminar pelo `Outcome`.
- Consts `OutcomeWin/OutcomeLoss/OutcomeDraw` no domínio do analytics (evita strings soltas).
- `GameStat` ganha `Losses int64` e `Draws int64`.

### D2 — Consumer parse
Lê `outcome` do envelope; se vazio, deriva de `won` (`won → win`, senão `loss`) e normaliza. Popula `PlayerResult.Outcome` (e mantém `Won`).

### D3 — Migration `002`
`ALTER TABLE game_rollup ADD COLUMN losses BIGINT NOT NULL DEFAULT 0`; idem `draws`. Aditivo, sem tabela nova.

### D4 — Repositório
- `ApplyMatch`: no laço dos resultados, `switch p.Outcome` incrementa `wins`/`losses`/`draws`; UPSERT dos 3 no `game_rollup`.
- `Overview`/`GameAnalytics`: `SELECT ... wins, losses, draws ...`; `Scan` nos novos campos.

### D5 — HTTP
`http_handler.go`: adiciona `losses`/`draws` ao JSON do jogo (aditivo).

## Arquivos

| Arquivo | Mudança |
|---|---|
| `internal/domain/analytics.go` | `PlayerResult.Outcome`; consts outcome; `GameStat.Losses/Draws` |
| `internal/consumer/consumer.go` | `parse` lê `outcome` (+fallback `won`) |
| `migrations/002_outcome_counts.sql` | `losses`/`draws` em `game_rollup` |
| `internal/repository/postgres_analytics.go` | conta por outcome; select/scan losses/draws |
| `internal/transport/http_handler.go` | JSON aditivo |
| testes | repo (win/loss/draw + idempotência), parse (outcome + fallback), handler (campos novos) |

## Ordem
domain → migration → consumer parse → repository → http → testes → build/`-race`/lint. (Smoke local exige recriar `analytics-postgres` — volume novo.)
