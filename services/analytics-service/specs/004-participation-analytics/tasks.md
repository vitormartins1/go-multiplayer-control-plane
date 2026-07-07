# Tasks: 004-participation-analytics

Fatia H2 (sink analytics) do `host_world_dropin` (ADR-0023). Consumer despacha por `kind`; agrega participações por `(gameId, role, event)`, idempotente por `(sessionID, playerID, event)`. Ver `plan.md` (D1–D6). Marcar `[x]` ao concluir.

Baseline: `go test ./services/analytics-service/... -race` **verde** antes.

## Domínio

- [x] **T1 — tipos + Ports.** `internal/domain/analytics.go`: `ParticipationEvent{GameID, SessionID, PlayerID, Role, Event, Outcome string}`; `ParticipationStat{Role, Event string; Count int64}`; `GameAnalytics.RoleParticipations []ParticipationStat`; `MatchRepository.ApplyParticipation(ctx, ParticipationEvent) (bool, error)`; `Ingestor.SubmitParticipation(ctx, ParticipationEvent) <-chan error`.

## Repositório

- [x] **T2 — migration + repo.** `migrations/003_participation_rollup.sql`: `processed_participations` (PK `(session_id, player_id, event)`) + `game_participation_rollup` (PK `(game_id, role, event)`, `count`). `postgres_analytics.go`: `ApplyParticipation` (tx idempotente + upsert `count = count + 1`); `GameAnalytics` lê `RoleParticipations`. Integração (`//go:build integration`): reprocessa → dup sem dobrar; `(role, event)` distintos acumulam; read reflete.

## Serviço

- [x] **T3 — pool polimórfico.** `internal/service/pool.go`: `job` += `participation *domain.ParticipationEvent`; `SubmitParticipation`; `process` ramifica (`ApplyParticipation` vs `ApplyMatch`). `pool_test.go`: caminho de participação com mock repo.

## Consumer

- [x] **T4 — dispatch por kind.** `internal/consumer/consumer.go`: `peekKind` + `handleBatch` ramifica — `participation_event` → `parseParticipation` + `SubmitParticipation`; `win_loss_draw`/v1 → `parse` + `Submit`; desconhecido → delete (descarta). `consumer_test.go`: roteamento.

## Transport

- [x] **T5 — JSON aditivo.** `internal/transport/http_handler.go`: `roleParticipations` no JSON do detalhe do jogo. `http_handler_test.go`: presente (com dados) e vazio (queue_arena).

## Fechamento

- [x] **T6 — verde + lint + docs.** `go test ./services/analytics-service/... -race -cover` (≥80% service/domain). `golangci-lint run`. `win_loss_draw` sem regressão. Atualizar a tabela de estado do README de game-models (H2 sink analytics) + consolidar follow-ups no doc de evolução.
