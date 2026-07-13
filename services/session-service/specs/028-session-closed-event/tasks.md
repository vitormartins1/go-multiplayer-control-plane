# Tasks — 028 evento de fechamento de sessão

- [x] 1. `internal/domain/session.go`: `SessionClosedPublisher` interface
- [x] 2. `internal/service/session_service.go`:
  - [x] 2.1. Campo `sessionClosedPublisher domain.SessionClosedPublisher`
  - [x] 2.2. `WithSessionClosedPublisher` Option
  - [x] 2.3. `publishSessionClosed(ctx, sess)` — nil-safe, best-effort,
        loga + incrementa contador em erro
  - [x] 2.4. `SessionClosedPublishFailures() float64`
  - [x] 2.5. `closeSession` chama `publishSessionClosed` após
        `persistHistory`
- [x] 3. `internal/publisher/sns_publisher.go`:
  - [x] 3.1. `kindSessionClosed` const
  - [x] 3.2. `sessionClosedEnvelope` struct
  - [x] 3.3. `PublishSessionClosed` — extrai archetype, formata
        timestamps, publica
- [x] 4. `internal/publisher/log_publisher.go`: `PublishSessionClosed`
      (fallback de dev)
- [x] 5. `cmd/server/main.go`: wire `WithSessionClosedPublisher`
      (`fanoutPublisher` ganhou a 3ª Port embutida)
- [x] 6. Métrica `session_closed_publish_failures_total` registrada no
      main (CounterFunc, mesmo padrão de
      `session_history_persist_failures_total`)
- [x] 7. Testes:
  - [x] 7.1. `closeSession` publica com o publisher configurado
  - [x] 7.2. Publisher nil -> closeSession funciona normalmente
  - [x] 7.3. Falha de publish -> closeSession completa mesmo assim,
        métrica incrementa
  - [x] 7.4. `sns_publisher_test.go`: envelope tem `kind`, `archetype`,
        `closeReason`, timestamps corretos (+ teste sem Profile ->
        archetype vazio)
- [x] 8. `go build ./...` + `go test ./... -race -cover` no session-service
      (210 passed, +5 novos)
- [x] 9. Validação ao vivo: fechado sessão `queue_arena` real via
      `ReportResult` (`close_reason=result`) e mundo `host_world_dropin`
      real via `HOST_KILLED` (`close_reason=host_killed` — o caso que
      ANTES nunca gerava nenhum evento no fan-out) — log "session closed
      publicado no tópico" confirmado nos dois casos, com `game_id`/
      `session_id`/`close_reason` corretos
