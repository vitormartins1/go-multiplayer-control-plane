# Tasks: 013-participation-events

Fatia H2 (lado session — produtor) do `host_world_dropin` (ADR-0023). Aditiva sobre H0/H1a. `ReportParticipationEvent` publica no `match-results` com `kind=participation_event` e aplica o efeito no slot. Ver `plan.md` (D1–D7). Marcar `[x]` ao concluir.

Baseline: `go test ./services/session-service/... -race` **verde** antes.

## Domínio

- [x] **T1 — tipos, Port, erro.** `internal/domain/session.go`: `ParticipationEvent{PlayerID, Role, Event, Outcome string}`; constantes `EventJoined/EventBossDefeated/EventHostKilled/EventDied/EventLeft` e `OutcomeSuccess/OutcomeFail/OutcomeNeutral`; `CloseReason` `CloseHostKilled = "host_killed"`; Port `ParticipationPublisher` (`PublishParticipation(ctx, gameID, sessionID, ParticipationEvent) error`); `SessionService.ReportParticipationEvent(ctx, sessionID, ParticipationEvent) error`. `errors.go`: `ErrInvalidParticipation`.

## Publisher

- [x] **T2 — PublishParticipation.** `internal/publisher/sns_publisher.go`: envelope `participation_event` (`schemaVersion`, `kind`, `gameId`, `sessionId`, `playerId`, `role`, `event`, `outcome`; **sem** `results`) publicado no mesmo `topicARN`; reusa `fanout_publish_total`. `log_publisher.go`: fallback que loga. Asserções `var _ domain.ParticipationPublisher = (*SNS)(nil)`/`(*Log)(nil)`. Teste (`sns_publisher_test.go`): o corpo publicado tem `kind=participation_event` e os campos, e não tem `results`.

## Service

- [x] **T3 — ReportParticipationEvent.** `internal/service/session_service.go`: `WithParticipationPublisher` (option + campo); `validateParticipation` (event/outcome no vocabulário; role não); `ReportParticipationEvent` = valida → acha mundo host_world aberto (`ErrWorldNotFound`) → **publica** (falha ⇒ erro, sem mutar) → efeito por evento (`DIED`/`LEFT` → `removeFromWorld` re-lookup sob lock, no-op se ausente; `HOST_KILLED` → `closeSession(host_killed)`; `JOINED`/`BOSS_DEFEATED` → nada); contador `ParticipationTotal(event)`. Testes (`participation_test.go`): cada evento + efeito, publish-first (falha não muta), inválidos, mundo inexistente.

## Proto + transport

- [x] **T4 — proto.** `proto/session/v1/session.proto`: RPC unary `ReportParticipationEvent(ReportParticipationEventRequest{session_id, player_id, role, event, outcome}) → ReportParticipationEventResponse{}`. `buf generate`.
- [x] **T5 — handler.** `internal/transport/grpc_server.go`: handler que traduz e mapeia erros (`ErrInvalidParticipation`→`InvalidArgument`, `ErrWorldNotFound`→`NotFound`, default→`Internal`). Teste bufconn: `LEFT` libera slot (some do `ListInvadableWorlds`/afeta ocupação), `HOST_KILLED` encerra o mundo, evento inválido → `InvalidArgument`.

## Wire-up + fechamento

- [x] **T6 — main.go.** `buildPublisher` devolve um valor que satisfaz `ResultPublisher` **e** `ParticipationPublisher`; passa na `WithParticipationPublisher`; métricas `participation_events_total{event}` (5×) e `sessions_closed_total{reason="host_killed"}`.
- [x] **T7 — verde + lint + docs.** `go test ./services/session-service/... -race -cover` (≥80% service/domain). `golangci-lint run`. queue_arena (001–010) + H0/H1a (011/012) sem regressão. GO-LEARNINGS se surgir algo. Atualizar a tabela de estado do README de game-models (H2 lado session).
