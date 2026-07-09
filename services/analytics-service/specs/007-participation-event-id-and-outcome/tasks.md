# Tasks: 007-participation-event-id-and-outcome

Follow-ups #4 (eventId) e #5 (outcome) do `host_world_dropin` (ADR-0023), lado analytics. Marcar `[x]` ao concluir. Ver `plan.md` (D1–D5).

**Pré-requisito: `session/021-participation-event-id` implementada** (envelope precisa carregar `eventId`).

Baseline: `go test ./services/analytics-service/... -race` **verde** antes de começar.

## Migration

- [x] **T1 — `006_participation_event_id_and_outcome.sql`.** Parte 1 (D2): `processed_participations` ganha `event_id` + backfill + PK nova. Parte 2: `game_participation_rollup` ganha `outcome` (default `''`) + PK nova `(game_id, role, event, outcome)`. Rodar isolada contra cópia com dados das migrations 003–005 pra confirmar zero perda.

## Domínio

- [x] **T2 — campos.** `internal/domain/analytics.go`: `ParticipationEvent` += `EventID string`; `ParticipationStat` += `Outcome string`.

## Repository

- [x] **T3 — `effectiveEventID`.** Mesma função de `achievements/012` (D1), duplicada aqui.
- [x] **T4 — `ApplyParticipation` com outcome na chave.** `internal/repository/postgres_analytics.go`: `processed_participations` usa `event_id` (com fallback); `game_participation_rollup` INSERT/`ON CONFLICT` inclui `outcome`.
- [x] **T5 — leitura com outcome.** `RoleParticipations`/query do `GameAnalytics`: `SELECT`/`ORDER BY` incluem `outcome`; scan preenche `ParticipationStat.Outcome`.

## Consumer

- [x] **T6 — parse do eventId.** `internal/consumer/consumer.go`: `parseParticipation` lê `eventId` do envelope.

## Testes

- [x] **T7 — casos novos.** Dois `eventId` diferentes (mesmo session/player/event) → rollup soma 2. `HOST_KILLED`·SUCCESS vs `HOST_KILLED`·FAIL → linhas separadas no rollup. `eventId` vazio → fallback determinístico.

## Fechamento

- [x] **T8 — verde + lint.** `go test ./services/analytics-service/... -race -cover` (baseline mantido). `go vet`. Confirmar `analytics/004`/`005`/`006` (placement/mission, mesma tabela-família de sessão) sem regressão.
- [x] **T9 — coordenar com o par.** Confirmar com `achievements/012` que o formato de `eventId` no envelope e o backfill determinístico batem (mesma fonte, `session/021`).
