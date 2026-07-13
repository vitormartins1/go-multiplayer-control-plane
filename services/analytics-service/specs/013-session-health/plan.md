# Plan — 013 session health

## Decisões técnicas

1. **Quarto tipo de job no worker pool existente**, não um pool separado —
   mesmo mecanismo de fan-in/fan-out (`job` struct ganha
   `sessionClosed *domain.SessionCloseEvent`, `process()` ganha um case),
   reaproveita toda a infraestrutura de concorrência/observabilidade já
   testada.
2. **`processed_sessions` como tabela de idempotência dedicada** (não
   reaproveita `processed_matches`) — `sessionId` não é `matchId`
   (`host_world_dropin` não tem matchId; `queue_arena` até tem, mas usar
   `sessionId` uniformemente evita um caso especial por arquétipo).
3. **PK `(game_id, mode_id, close_reason)`** — sem `archetype` na PK: um
   modo só tem UM arquétipo (invariante do game-registry), então
   `archetype` é redundante como chave, mas útil como coluna (evita um
   JOIN pro admin-ui saber o arquétipo sem consultar o game-registry de
   novo). Grava-se a cada UPSERT, sempre o mesmo valor pro mesmo modo.
4. **`AvgDurationMs` computado no read** (`DurationSumMs / Count`), mesmo
   padrão de `StatSummary.Avg` — a tabela guarda só soma+contagem.
5. **Sem série diária nesta v1** — escopo deliberadamente menor que
   `PlacementDaily`/`MissionDaily`/`ParticipationDaily` (spec 010): a
   pergunta que motivou o item C é sobre TAXA e MÉDIA (dado cumulativo),
   não sobre tendência ao longo do tempo. Adicionar depois é aditivo puro
   (nova tabela, novo campo em `GameAnalytics`) se houver demanda real.

## Arquivos por camada

- `migrations/010_session_health.sql` (novo): `processed_sessions` +
  `game_session_rollup` (schema do spec.md).
- `internal/domain/analytics.go`:
  - `SessionCloseEvent`, `SessionCloseStat` (novos).
  - `MatchRepository.ApplySessionClosed`.
  - `Ingestor.SubmitSessionClosed`.
  - `GameAnalytics.SessionHealth []SessionCloseStat`.
- `internal/consumer/consumer.go`:
  - `kindSessionClosed = "session_closed"` — novo `case` em `handleBatch`
    (antes do `default`, mesmo nível dos outros 3 kinds com kind
    explícito).
  - `parseSessionClosed(body []byte) (domain.SessionCloseEvent, error)`.
- `internal/service/pool.go`:
  - `job.sessionClosed *domain.SessionCloseEvent`.
  - `SubmitSessionClosed` (mesmo formato dos outros 3 `Submit*`).
  - `process()`: novo `case j.sessionClosed != nil:` chamando
    `p.repo.ApplySessionClosed`.
- `internal/repository/postgres_analytics.go`:
  - `ApplySessionClosed`: mesma forma de `ApplyMatch`/`ApplyPlacement`
    (mark em `processed_sessions` + `ON CONFLICT DO NOTHING` +
    `RowsAffected()==0` -> dup; senão UPSERT em `game_session_rollup` com
    `duration_ms = closedAt - openedAt` calculado em Go antes do INSERT).
  - `GameAnalytics`: nova query `SELECT mode_id, archetype, close_reason,
    count, duration_sum_ms FROM game_session_rollup WHERE game_id = $1 AND
    ($2 = '' OR mode_id = $2)` (mesmo padrão `AND ($N='' OR...)` já usado
    nas 3 séries + stats da spec 012), preenche `AvgDurationMs` no loop de
    scan.
- `internal/transport/http_handler.go`: `gameAnalyticsDTO` ganha
  `SessionHealth []sessionCloseStatDTO`.
- Testes: `consumer_test.go` (`parseSessionClosed`), `postgres_analytics_test.go`
  (integration: idempotência, filtro por modo, 2 motivos distintos),
  `http_handler_test.go` (DTO).

## Ordem de implementação

Migration → domínio → consumer (parse + dispatch) → pool (job + Submit +
process) → repository (`ApplySessionClosed` + leitura em `GameAnalytics`)
→ transport (DTO) → testes → build/test completo → validação ao vivo
(depende do session-service/028 publicando de verdade — fechar sessões
reais dos 4 arquétipos, conferir `sessionHealth` na resposta).
