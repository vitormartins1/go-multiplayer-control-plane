# Tasks: 003-outcome-aggregation

- [x] **T1** — `internal/domain/analytics.go`: `PlayerResult.Outcome string`; consts `OutcomeWin/OutcomeLoss/OutcomeDraw`; `GameStat.Losses/Draws int64`.
- [x] **T2** — `internal/consumer/consumer.go`: `parse` lê `outcome` do envelope; fallback para `won` quando vazio; popula `PlayerResult.Outcome`.
- [x] **T3** — `migrations/002_outcome_counts.sql`: `game_rollup` ganha `losses`/`draws` (BIGINT NOT NULL DEFAULT 0).
- [x] **T4** — `internal/repository/postgres_analytics.go`: `ApplyMatch` conta por `Outcome` (wins/losses/draws) e faz UPSERT dos 3; `Overview`/`GameAnalytics` selecionam e escaneiam `losses`/`draws`.
- [x] **T5** — `internal/transport/http_handler.go`: expõe `losses`/`draws` no JSON (aditivo).
- [x] **T6** — Testes repo: partida win/loss/draw agrega certo; `wins+losses+draws==participations`; idempotência por matchID; empate ≠ derrota.
- [x] **T7** — Testes consumer parse: envelope com `outcome`; envelope só com `won` (fallback).
- [x] **T8** — `go build ./... && go test ./services/analytics-service/... -race -cover`; `golangci-lint run`.
- [x] **T9** — Aprendizado (CLAUDE.md): GO-LEARNINGS se houver algo novo; senão dizer que não houve.
