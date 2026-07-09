# Tasks: 021-participation-event-id

Follow-up #4 (eventId) do `host_world_dropin` (ADR-0023), lado session. Passthrough puro, aditivo sobre `session/013`. Marcar `[x]` ao concluir. Ver `plan.md` (D1–D4).

Baseline: `go test ./services/session-service/... -race` **verde** antes de começar.

## Domínio

- [x] **T1 — campo.** `internal/domain/session.go`: `ParticipationEvent` += `EventID string`.

## Publisher

- [x] **T2 — envelope.** `internal/publisher/sns_publisher.go`: `participationEnvelope` += `EventID string \`json:"eventId,omitempty"\``; `PublishParticipation` (SNS) repassa `evt.EventID`. `internal/publisher/log_publisher.go`: idem (loga o campo).

## Proto + transport

- [x] **T3 — proto.** `proto/session/v1/session.proto`: conferir índice livre real em `ReportParticipationEventRequest` (estimado 6); adicionar `string event_id`. `PATH="$(go env GOPATH)/bin:$PATH" buf generate`.
- [x] **T4 — handler.** `internal/transport/grpc_server.go`: `ReportParticipationEvent` passa `req.GetEventId()`. Teste: `event_id` chega íntegro no envelope publicado (fake publisher captura o `ParticipationEvent`).

## Fechamento

- [x] **T5 — verde + lint.** `go test ./services/session-service/... -race -cover` (≥80% service/domain mantido). `go vet`. Confirmar `session/011–013` sem regressão.
- [x] **T6 — coordenar com os pares.** Confirmar com `achievements/012` e `analytics/007` que o campo se chama `eventId` no JSON do envelope antes deles implementarem o consumo.
