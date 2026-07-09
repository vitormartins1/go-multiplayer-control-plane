# Tasks: 012-participation-event-id

Follow-up #4 (eventId) do `host_world_dropin` (ADR-0023), lado achievements. Marcar `[x]` ao concluir. Ver `plan.md` (D1–D4).

**Pré-requisito: `session/021-participation-event-id` implementada** (envelope precisa carregar `eventId`).

Baseline: `go test ./services/achievements-service/... -race` **verde** antes de começar.

## Migration

- [x] **T1 — `007_participation_event_id.sql`.** `ADD COLUMN event_id`; backfill determinístico (`session_id||':'||player_id||':'||event`); PK nova em `event_id` (D2). Rodar isolada contra uma cópia com dados da 006 pra confirmar zero perda.

## Domínio / repository

- [x] **T2 — `effectiveEventID`.** Nova função (D1): `eventID` não-vazio → usa direto; vazio → sintetiza `session_id:player_id:event`.
- [x] **T3 — `ApplyParticipationEvent` usa `event_id` na PK.** `internal/repository/postgres_achievement.go`: assinatura ganha `eventID string` (D3); INSERT/`ON CONFLICT` em `event_id`.

## Service + consumer

- [x] **T4 — assinatura propagada.** `internal/service/achievement_service.go` (e a Port, se declarada em `domain/`): repassa `eventID`.
- [x] **T5 — consumer lê o campo.** `cmd/consumer/main.go`: `handleParticipation` parseia `eventId` do envelope, chama o service com o valor.

## Testes

- [x] **T6 — casos novos.** Dois `eventId` diferentes (mesmo session/player/event) → conta 2×. Mesmo `eventId` (retry) → no-op. `eventId` vazio → dedupe por `(session,player,event)` como antes.

## Fechamento

- [x] **T7 — verde + lint.** `go test ./services/achievements-service/... -race -cover` (baseline de cobertura mantido). `go vet`. Confirmar `achievements/009` (specs anteriores) sem regressão.
- [x] **T8 — coordenar com o par.** Confirmar com `analytics/007` o mesmo formato de migration/backfill (mesma tabela `processed_participations`, serviços diferentes/bancos diferentes, mas mesmo shape).
