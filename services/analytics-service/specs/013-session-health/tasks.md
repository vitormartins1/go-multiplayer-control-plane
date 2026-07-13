# Tasks — 013 session health

- [x] 1. `migrations/010_session_health.sql`: `processed_sessions` +
      `game_session_rollup`
- [x] 2. `internal/domain/analytics.go`:
  - [x] 2.1. `SessionCloseEvent`, `SessionCloseStat`
  - [x] 2.2. `MatchRepository.ApplySessionClosed`
  - [x] 2.3. `Ingestor.SubmitSessionClosed`
  - [x] 2.4. `GameAnalytics.SessionHealth`
- [x] 3. `internal/consumer/consumer.go`:
  - [x] 3.1. `case kind == "session_closed"` em `handleBatch`
  - [x] 3.2. `parseSessionClosed`
- [x] 4. `internal/service/pool.go`:
  - [x] 4.1. `job.sessionClosed`
  - [x] 4.2. `SubmitSessionClosed`
  - [x] 4.3. `process()` novo case
- [x] 5. `internal/repository/postgres_analytics.go`:
  - [x] 5.1. `ApplySessionClosed` (idempotente, UPSERT com soma de
        duração)
  - [x] 5.2. `GameAnalytics`: query de `SessionHealth` com filtro por
        `modeID`
- [x] 6. `internal/transport/http_handler.go`: DTO `SessionHealth`
- [x] 7. Atualizar `internal/service/pool_test.go` (`fakeRepo`) pra nova
      interface
- [x] 8. Testes:
  - [x] 8.1. `consumer_test.go`: `parseSessionClosed` (campos +
        timestamp inválido -> erro)
  - [x] 8.2. Repository (integration): idempotência por sessionId
        (`TestApplySessionClosed_Reprocessado_NaoDobra`); 2 `closeReason`
        distintos -> 2 linhas (`TestApplySessionClosed_GravaRollupPorMotivo`);
        filtro por `modeId` (`TestGameAnalytics_SessionHealth_FiltraPorModo`)
  - [x] 8.3. Transport: DTO expõe `sessionHealth` (presente e vazio)
- [x] 9. `go build ./...` + `go test ./... -race -cover` (39 unit + 31
      integration, todos passando)
- [x] 10. Validação ao vivo (depende do session-service/028, já mergeado):
      partida `queue_arena` real fechada por `ReportResult` ->
      `sessionHealth` mostra `default`/`result`/count=1/duração real
      (8s); mundo `host_world_dropin` real fechado por `HOST_KILLED` ->
      `sessionHealth` mostra `mundo-vivo`/`host_killed`/count=1 (o caso
      que antes NUNCA aparecia no fan-out); `?modeId=default` isola
      corretamente
