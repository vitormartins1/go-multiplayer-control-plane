# Plan: 013-participation-events

**Serviço:** session-service · **Spec:** `spec.md` (Aprovada 2026-07-07) · **ADR:** 0023 (H2, lado session — produtor)
**Depende de:** `session/011` (mundo host-cêntrico, slots) + o fan-out do `queue_arena` (ADR-0020).

Como o session ganha `ReportParticipationEvent` (publica no `match-results` com `kind=participation_event` e aplica o efeito no slot). Fatia **aditiva** — `ReportResult`/`queue_arena` e H0/H1a não regridem.

## Decisões técnicas

### D1 — Port nova `ParticipationPublisher`, o mesmo adapter satisfaz as duas

O `ResultPublisher.Publish(ctx, gameID, matchID, MatchOutcome)` (queue_arena) fica **intacto**. Publicar participação precisa de outra forma (sem matchID/outcome; com sessionID/role/event). Nova Port de saída:

```go
type ParticipationPublisher interface {
    PublishParticipation(ctx context.Context, gameID, sessionID string, evt ParticipationEvent) error
}
```

- Implementada pelos **mesmos** adapters `publisher.SNS` e `publisher.Log` (um struct, duas Ports — padrão já usado no matchmaking). O SNS serializa o **envelope `participation_event`** e publica no **mesmo** `topicARN`; o Log faz o fallback de dev.
- Reusa o `fanout_publish_total{result}` do SNS para ok/erro.

### D2 — o service recebe a Port via Option (sem mexer na assinatura de `New`)

`service.New(alloc, issuer, pub, opts...)` é chamado em muitos testes — não muda. Nova `WithParticipationPublisher(p domain.ParticipationPublisher)`. Campo `participation` no `Service`; se nil, `ReportParticipationEvent` responde erro (mundo host_world sem fan-out configurado é bug de wire-up — como o `profiles` nil no OpenWorld). No `main`, `buildPublisher` passa a devolver um tipo que satisfaz **as duas** Ports (interface combinada local), e o mesmo valor vai como `pub` posicional e na option.

### D3 — `ReportParticipationEvent`: valida → acha mundo → publica → muta slot

```go
func (s *Service) ReportParticipationEvent(ctx, sessionID string, evt domain.ParticipationEvent) error {
    // 1. valida vocabulário (event/outcome); role não é revalidado (spec).
    // 2. sob o lock: acha a sessão host_world aberta; senão ErrWorldNotFound. Lê gameID.
    // 3. PUBLICA (participation) — falha => retorna erro, NÃO muta (server refaz).
    // 4. efeito no slot por evento (re-lookup sob o lock; tolerante a replay).
}
```

- **Ordem publica→muta** (ADR-0020, igual ao `ReportResult`): a mutação só acontece após o publish ok.
- **Efeito por evento** (re-busca a sessão sob o lock antes de mutar, pois entre o publish e a mutação outra goroutine pode ter fechado o mundo):
  - `DIED`/`LEFT` → `removeFromWorld(sess, playerID)` (no-op se ausente — idempotente).
  - `HOST_KILLED` → `closeSession(sessionID, domain.CloseHostKilled)` (reusa o fechamento unificado; `ErrSessionNotFound` ⇒ já fechado ⇒ traduz para no-op/`ErrWorldNotFound` conforme o caso).
  - `JOINED`/`BOSS_DEFEATED` → nenhum.
- **Reuso:** `removeFromWorld` e `closeSession` já existem (H0). Nada novo de concorrência.

### D4 — validação do vocabulário: função pequena no domínio ou no service

`validateParticipation(evt)`: `event ∈ {JOINED,BOSS_DEFEATED,HOST_KILLED,DIED,LEFT}` e `outcome ∈ {SUCCESS,FAIL,NEUTRAL}` senão `ErrInvalidParticipation`. `role` não validado (o server é autoritativo; spec). Vocabulário como constantes no domínio (`EventJoined`… / `OutcomeSuccess`…), como o `queue_arena` fez com `ResultWin`….

### D5 — proto aditivo + `buf`

- Novo RPC unary `ReportParticipationEvent(ReportParticipationEventRequest{string session_id; string player_id; string role; string event; string outcome}) → ReportParticipationEventResponse{}`.
- Regenerar com `buf` (nunca `sed`).

### D6 — transport

- Handler `ReportParticipationEvent`: traduz o request para `domain.ParticipationEvent`, chama o service, mapeia erros → `ErrInvalidParticipation`→`InvalidArgument`, `ErrWorldNotFound`→`NotFound`, publish falho→`Internal`. Reusa o `worldError` (adiciona o caso `ErrInvalidParticipation`) ou um mapeamento próprio.

### D7 — observabilidade

- `participation_events_total{event}` — CounterFunc no `main` por evento (5×, ConstLabels, padrão do `sessions_closed_total`), lendo `svc.ParticipationTotal(event)`. Contador em memória no service (`map[string]int64` sob o lock, como `closed`).
- `sessions_closed_total{reason="host_killed"}` — mais um CounterFunc para o novo motivo.

## Domínio (`internal/domain/`)

- `session.go`: `ParticipationEvent{PlayerID, Role, Event, Outcome string}`; constantes de evento/outcome; `SessionService.ReportParticipationEvent(ctx, sessionID, ParticipationEvent) error`; `ParticipationPublisher` Port; `CloseReason` `CloseHostKilled = "host_killed"`.
- `errors.go`: `ErrInvalidParticipation`.

## Arquivos a tocar

| Arquivo | Mudança |
|---|---|
| `internal/domain/session.go` | `ParticipationEvent`, consts, Port, `CloseHostKilled`, assinatura na `SessionService` |
| `internal/domain/errors.go` | `ErrInvalidParticipation` |
| `internal/service/session_service.go` | `ReportParticipationEvent` + `validateParticipation` + contador + `WithParticipationPublisher` |
| `internal/service/participation_test.go` (novo) | ramos (cada evento, efeito no slot, publish-first, inválidos) |
| `internal/publisher/sns_publisher.go` | `PublishParticipation` + envelope `participation_event` |
| `internal/publisher/log_publisher.go` | `PublishParticipation` (fallback) |
| `internal/publisher/sns_publisher_test.go` | teste do envelope de participação |
| `proto/session/v1/session.proto` | RPC + messages; `buf generate` |
| `internal/transport/grpc_server.go` | handler `ReportParticipationEvent` + erro |
| `internal/transport/*_grpc_test.go` | e2e via bufconn |
| `cmd/server/main.go` | `buildPublisher` devolve as 2 Ports; `WithParticipationPublisher`; métricas (`participation_events_total`, `sessions_closed_total{host_killed}`) |

## Testes

- **Baseline** `go test ./services/session-service/... -race` verde antes.
- `service` (mock de `ParticipationPublisher` + os mocks da H0): cada evento publica; `DIED`/`LEFT` libera slot (e no-op se ausente); `HOST_KILLED` fecha (`host_killed`, server liberado); `JOINED`/`BOSS_DEFEATED` não mexem no slot; event/outcome inválido → `ErrInvalidParticipation` (sem publicar); mundo inexistente → `ErrWorldNotFound`; **publish falho → não muta** (slot intacto / mundo aberto); contador por evento.
- `publisher`: o envelope `participation_event` carrega `kind`/`sessionId`/`playerId`/`role`/`event`/`outcome` e **não** carrega `results`.
- `transport` (bufconn): report de cada tipo → efeito observável (mundo some no `HOST_KILLED`; slot livre no `LEFT`); inválido → `InvalidArgument`.
- Cobertura ≥80% em `service/`/`domain/`.

## Sequência de tasks (preview)

1. Baseline verde.
2. Domínio: `ParticipationEvent` + consts + Port + `CloseHostKilled` + `ErrInvalidParticipation` + assinatura.
3. Publisher: `PublishParticipation` no SNS (envelope) e no Log (+ teste do envelope).
4. Service: `ReportParticipationEvent` + `validateParticipation` + `WithParticipationPublisher` + contador (+ testes).
5. Proto + `buf` + transport (handler) + testes bufconn.
6. Wire-up `main` (buildPublisher 2 Ports, option, métricas).
7. `go test ./... -race` verde; lint. GO-LEARNINGS se surgir algo (candidato: envelope discriminado por `kind` no mesmo tópico — o produtor do lado do fan-out; publica-antes-de-mutar). Atualizar a tabela de estado do README (H2 lado session).

## Riscos / gotchas

- **Publica antes de mutar** — se inverter, um replay do server (após publish ok mas antes da resposta) poderia dobrar o efeito de slot; a ordem certa + idempotência do `removeFromWorld` cobrem.
- **Re-lookup sob o lock** antes de mutar (o mundo pode ter fechado entre o publish e a mutação).
- **`buf generate`**, nunca `sed`. Sem migration (sessão em memória).
- **Não** regredir os consumers: esta fatia só publica; o dispatch por `kind` nos sinks é sub-fatia seguinte (spec deixa explícito).
