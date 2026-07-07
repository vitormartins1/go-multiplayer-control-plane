# Plan: 009-participation-consumer

**Serviço:** achievements-service · **Spec:** `spec.md` (Aprovada 2026-07-07) · **ADR:** 0023 (H2, sink)
**Depende de:** `session/013` (envelope `participation_event`) + A1/008 (critério por `event_type`, reusado).

Como o achievements passa a **despachar por `kind`** e a reagir aos eventos de participação por papel. Fatia **aditiva** — o caminho `win_loss_draw` (`ApplyMatchResult`) não regride.

## Decisões técnicas

### D1 — dispatch por `kind` no `makeHandler` (cmd/consumer)

O handler decodifica primeiro `struct{ Kind string }` e ramifica:

- `kind == "win_loss_draw"` **ou** ausente **com** `matchId` não-vazio → decodifica `{matchId, gameId, results}` e chama `svc.ApplyMatchResult` (comportamento atual, retrocompat com o envelope v1 que não tinha `kind`).
- `kind == "participation_event"` → decodifica `{gameId, sessionId, playerId, role, event, outcome}` e chama `svc.ApplyParticipationEvent`.
- qualquer outro → log em `debug` + `return false, nil` (descarta; não vai à DLQ, não polui `processed_matches`).

### D2 — `ParticipationEventTypes` (domínio, função pura)

`internal/domain/`: função `ParticipationEventTypes(role, event, outcome string) []string` + constantes dos event types de gameplay:

```go
const (
    EventInvasionWon    = "invasion_won"
    EventCoopBossHelped = "coop_boss_helped"
    EventWorldJoined    = "world_joined"
)
```

Mapa (spec): `invader`+`HOST_KILLED`+`SUCCESS`→`invasion_won`; `coop`+`BOSS_DEFEATED`+`SUCCESS`→`coop_boss_helped`; qualquer `JOINED`→`world_joined`; resto→`nil`. Pura e testável em tabela.

### D3 — `Service.ApplyParticipationEvent` (tradução → repo)

```go
func (s *Service) ApplyParticipationEvent(ctx, gameID, sessionID, playerID, role, event, outcome string) (bool, error) {
    eventTypes := domain.ParticipationEventTypes(role, event, outcome)
    return s.repo.ApplyParticipationEvent(ctx, gameID, sessionID, playerID, event, eventTypes, s.now().UTC())
}
```

- Concreto no `*Service` (como `ApplyMatchResult`, não na Port de entrada — o consumer chama o struct).
- `eventTypes` vazio → o repo marca processado e não aplica progresso (no-op idempotente).

### D4 — `repo.ApplyParticipationEvent` + `processed_participations`

Novo método na Port `AchievementRepository` e no Postgres:

```go
ApplyParticipationEvent(ctx, gameID, sessionID, playerID, event string, eventTypes []string, at time.Time) (dup bool, err error)
```

- Tx única: `INSERT INTO processed_participations (session_id, player_id, event, game_id) VALUES (...) ON CONFLICT DO NOTHING`; se `RowsAffected()==0` → `dup=true`, commit (no-op). Senão, para cada `eventType`: `findByEventType` + `recordProgress` (helpers de tx já existentes — reuso direto, como o `ApplyMatchResult`). Commit.
- **Migration `006_processed_participations.sql`** (PK `(session_id, player_id, event)`), espelhando `004_processed_matches.sql`.
- Mock do repo no teste do service ganha o método.

### D5 — observabilidade

- Reusa o contador de mensagens do consumer; opcional um label/contador `participation_events_consumed_total{event}`. Decisão do plan: **manter simples** — o consumer já tem métricas RED; um contador dedicado pode ficar para a H3 (funil no dashboard). Registro como pendência, não bloqueia.

## Arquivos a tocar

| Arquivo | Mudança |
|---|---|
| `internal/domain/achievement.go` | `ParticipationEventTypes` + consts; `ApplyParticipationEvent` na Port `AchievementRepository` |
| `internal/domain/participation_test.go` (novo) | tabela de `ParticipationEventTypes` |
| `internal/service/achievement_service.go` | `ApplyParticipationEvent` (concreto) |
| `internal/service/achievement_service_test.go` | mock repo += método; testes do service (tradução + no-op) |
| `internal/repository/postgres_achievement.go` | `ApplyParticipationEvent` (tx idempotente) |
| `internal/repository/postgres_achievement_test.go` | teste de integração (`//go:build integration`) |
| `migrations/006_processed_participations.sql` (novo) | tabela de idempotência |
| `cmd/consumer/main.go` | dispatch por `kind` no `makeHandler` |

## Testes

- **Baseline** `go test ./services/achievements-service/... -race` verde antes.
- `domain`: `ParticipationEventTypes` em tabela (cada linha do mapa + eventos sem tradução → nil).
- `service` (mock repo): `ApplyParticipationEvent` traduz e delega com os event types certos; `LEFT`/`DIED` → delega com lista vazia (repo marca processado); `dup` propaga.
- `consumer` (unit do `makeHandler`, se houver harness; senão coberto pelo teste do consumer existente): `kind=participation_event` roteia para `ApplyParticipationEvent`; `kind` desconhecido → descarta; `win_loss_draw` → `ApplyMatchResult`.
- `repository` (integration, gated): reprocessar o mesmo `(session, player, event)` → `dup`, sem dobrar; `(session, player, event)` distintos acumulam; event type sem conquista → sem progresso mas processado.
- Cobertura ≥80% em `service/`/`domain/`.

## Sequência de tasks (preview)

1. Baseline verde.
2. Domínio: `ParticipationEventTypes` + consts + método na Port (+ teste tabela).
3. Migration `006` + repo `ApplyParticipationEvent` (+ teste integration).
4. Service `ApplyParticipationEvent` + mock repo no teste (+ testes service).
5. Consumer: dispatch por `kind` no `makeHandler` (+ teste se houver).
6. `go test ./... -race` verde; lint. Atualizar a tabela de estado do README (H2 sink achievements).

## Riscos / gotchas

- **Não regredir `win_loss_draw`:** o dispatch precisa manter o envelope v1 (sem `kind`, com `matchId`) indo para `ApplyMatchResult`.
- **`kind` desconhecido é descarte, não erro** — senão vai à DLQ e trava a fila.
- **Integração gated:** o teste de repo roda só com `-tags=integration` + Docker; o verde padrão cobre domínio+service.
- Sem `buf` (achievements é HTTP/consumer, sem proto de entrada aqui).
