# Plan: 021-participation-event-id

**Serviço:** session-service · **Spec:** `spec.md` (Proposta 2026-07-09) · **ADR:** 0023 (follow-up #4, lado session)

Passthrough puro — sem lógica nova, só um campo a mais no envelope. Fatia **aditiva** sobre `session/013`.

## Decisões técnicas

### D1 — `event_id` nasce no chamador (data plane), não aqui (ver spec.md "Contexto")

Repetido aqui por ênfase: **não** gerar `uuid.New()` dentro do handler `ReportParticipationEvent`. Isso quebraria a garantia de retry-safety que `(session_id, player_id, event)` já dava — um `event_id` gerado na recepção seria diferente a cada tentativa de retry do client, mesmo que seja logicamente "a mesma ocorrência".

### D2 — proto aditivo + `buf`

`ReportParticipationEventRequest` += `string event_id = 6;`. Conferir o índice livre real antes de gerar (a spec estima 6, com 1–5 ocupados).

### D3 — domínio e publisher

- `ParticipationEvent` (domain) += `EventID string`.
- `participationEnvelope` (publisher/sns_publisher.go) += `EventID string \`json:"eventId,omitempty"\`` — `omitempty` porque data plane antigo (pré spec 004 do Unity) pode mandar vazio, e não faz sentido publicar `"eventId":""` explicitamente.
- `PublishParticipation` (SNS e Log) só repassam `evt.EventID` pro envelope — sem lógica.

### D4 — transport

- Handler `ReportParticipationEvent`: `domain.ParticipationEvent{..., EventID: req.GetEventId()}`.

## Arquivos a tocar

| Arquivo | Mudança |
|---|---|
| `internal/domain/session.go` | `ParticipationEvent.EventID` |
| `proto/session/v1/session.proto` | campo 6; `buf generate` |
| `internal/publisher/sns_publisher.go` | `participationEnvelope.EventID`; `PublishParticipation` repassa |
| `internal/publisher/log_publisher.go` | loga o `EventID` também (debug) |
| `internal/transport/grpc_server.go` | handler passa `req.GetEventId()` |
| `internal/transport/host_world_grpc_test.go` | teste: `event_id` chega no envelope (via fake publisher que captura o evento) |

`cmd/server/main.go` — sem mudança.

## Testes

- Baseline `go test ./services/session-service/... -race` verde antes.
- `publisher`: `EventID` passa pro envelope (SNS e Log), vazio não quebra nada.
- `transport`: `ReportParticipationEvent` com `event_id` → o fake/mock do publisher recebe o campo certo.
- Cobertura ≥80% em `service/`/`domain/` (mantida — mudança pequena, não deveria cair).

## Sequência de tasks (preview)

1. Baseline verde.
2. Domínio: `EventID` em `ParticipationEvent`.
3. Publisher: envelope + repasse (SNS e Log).
4. Proto (campo 6) + `buf generate` + handler transport + teste.
5. `go test ./... -race` verde; lint.

## Riscos / gotchas

- **Não inverter D1** — é a decisão mais importante desta spec; qualquer implementação futura que "simplifique" gerando o ID aqui quebra retry-safety silenciosamente (só aparece como contagem duplicada esporádica sob falha de rede, difícil de reproduzir em teste determinístico).
- Índice de proto: conferir antes de gerar.
- Sem migration (session não persiste os eventos).
