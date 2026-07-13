# Tasks — 027 resultado e outcome no histórico

- [x] 1. Migration `migrations/003_closed_session_result_and_outcome.sql`
      (`ALTER TABLE closed_sessions ADD COLUMN result JSONB;` +
      `ALTER TABLE closed_sessions ADD COLUMN team_by_player JSONB;` +
      `ALTER TABLE closed_session_players ADD COLUMN last_event TEXT NOT NULL
      DEFAULT '';` + `... last_outcome TEXT NOT NULL DEFAULT '';`)
- [x] 2. `internal/domain/session.go` — `Participant` ganha `LastEvent`,
      `LastOutcome string`
- [x] 3. `internal/service/session_service.go`:
  - [x] 3.1. Helper `setParticipantOutcome(participants []domain.
        Participant, playerID, event, outcome string) bool` (retorna se
        achou/atualizou; no-op se não achar)
  - [x] 3.2. Chamar em `ReportParticipationEvent`, com um `s.mu.Lock()`
        DEDICADO (não há lock aberto nesse ponto — o primeiro foi liberado
        antes do publish), após `s.countParticipation(...)` e antes do
        `switch evt.Event`; re-lookup de `sess` sob o lock (idempotente, a
        sessão pode ter fechado nesse meio-tempo — mesmo padrão do case
        `Died`/`Left`)
  - [x] 3.3. `upsertParticipant`: no branch de troca de papel (`if p.Role !=
        role`), zerar também `LastEvent`/`LastOutcome`
- [x] 4. `internal/repository/postgres_history.go`:
  - [x] 4.1. `SaveClosed`: serializar `sess.Result` e `sess.TeamByPlayer`
        (`json.Marshal`, `nil` vira `NULL`) nas colunas `result`/
        `team_by_player`; INSERT de participantes inclui `last_event`/
        `last_outcome`
  - [x] 4.2. `scanClosedSessions` (ou equivalente): ler `result` e
        `team_by_player` (`[]byte`/nullable), `json.Unmarshal` se não-nulo,
        atribuir em `domain.Session.Result`/`domain.Session.TeamByPlayer`
  - [x] 4.3. `attachParticipants` (ou equivalente): ler `last_event`/
        `last_outcome`, atribuir no `domain.Participant`
- [x] 5. `internal/transport/http_handler.go`:
  - [x] 5.1. `participantDTO` ganha `LastEvent`/`LastOutcome string` com
        `json:"lastEvent,omitempty"`/`json:"lastOutcome,omitempty"`
  - [x] 5.2. Conversão pra DTO (onde `participantDTO` é montado a partir de
        `domain.Participant`) propaga os campos novos
- [x] 6. Testes:
  - [x] 6.1. `session_service_test.go`/`participation_test.go`: evento
        atualiza LastEvent/Outcome; participante inexistente é no-op; DIED
        libera slot sem perder o outcome; troca de papel reseta, mesmo papel
        não reseta
  - [x] 6.2. `postgres_history_test.go`: round-trip de `result` e
        `team_by_player` (presentes e `NULL`) e de `last_event`/
        `last_outcome` — 3 testes novos, rodados com `-tags=integration`
        (testcontainers, Docker real)
  - [x] 6.3. `http_handler_test.go`: DTO expõe os campos novos quando
        presentes, omite quando vazios (`omitempty`)
- [x] 7. `go build ./...` + `go test ./... -race -cover` no session-service
      (205 passed) + `go test -tags=integration ./internal/repository/...`
      (8 passed, Docker real)
- [x] 8. Validação ao vivo (stack docker compose, migration 003 aplicada
      manualmente no volume existente, container rebuildado):
  - [x] 8.1. Mundo `host_world_dropin` real (Achievements Test/mundo-vivo):
        `OpenWorld` + `JoinWorld` coop + `ReportParticipationEvent`
        DIED/FAIL + `HOST_KILLED` (fecha o mundo) -> `GET .../sessions?
        state=closed` devolve `lastEvent: "DIED"`, `lastOutcome: "FAIL"`
        pro coop; host sem os campos (omitido, nunca reportou evento)
  - [x] 8.2. Partida `queue_arena` teams-2v2 real (Packet Arena):
        `CreateSession` + `ReportResult` win_loss_draw (4 jogadores, 2
        times) -> `result` presente no histórico; RESTART real do
        session-service (`docker compose restart session-service`) ->
        `result` sobrevive (confirmado via GET pós-restart)
  - [x] 8.3. Sessões fechadas sem resultado (`ttl`, várias sessões
        `host_world_dropin` reais na mesma listagem) -> `result` ausente
        (omitido no JSON), sem erro
  - [x] 8.4. Mesma partida do item 8.2: `teams` presente no histórico
        persistido, sobrevive ao mesmo restart. Renderização do `TeamCards`
        no admin-ui fica para a fatia `admin-ui/063` (ainda não
        implementada nesta etapa).
