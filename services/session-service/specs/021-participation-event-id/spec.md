# Spec: 021-participation-event-id

**Serviço:** session-service
**Status:** Proposta (2026-07-09)
**Data:** 2026-07-09
**ADR:** 0023 — **arquétipo `host_world_dropin`, follow-up #4** (lado session: campo `eventId` no envelope)
**Roadmap:** `docs/game-models/host_world_dropin-follow-ups.md` → item 4 (`eventId` único por ocorrência)
**Emparelhada com:** `achievements-service/specs/012-participation-event-id` e `analytics-service/specs/007-participation-event-id-and-outcome` (consumidores), `unity-host-world-dropin-data-plane/docs/specs/004-boss-combat-lifecycle` (Rodada 3 — o boss pode morrer várias vezes na mesma sessão, o cenário que evidencia este follow-up)

## Contexto

Hoje o produtor (`ReportParticipationEvent`) não carrega um identificador único por ocorrência — os sinks (`achievements/009`, `analytics/004`) dedupe por `(session_id, player_id, event)`, contando **uma vez por tipo de evento por sessão por jogador**. Consequência prática (evidenciada pela spec 004 do Unity, onde o boss respawna): "ajudou N bosses na mesma sessão" hoje = N **sessões**, não N bosses numa sessão — o segundo `BOSS_DEFEATED` do mesmo co-op na mesma sessão é descartado como duplicata.

**Decisão de design que não estava explícita no follow-up original:** o `eventId` **não pode ser gerado pelo session-service ao receber a chamada** — isso quebraria a idempotência de retry que já existe. O client (`SessionRpcClient.ReportParticipationEventAsync`, Unity) já reenvia até 3× em falha de rede (retry ≤3, spec 001); se o session gerasse um `eventId` novo a cada chamada recebida, uma resposta perdida (mas a chamada já processada) faria o retry criar um `eventId` **diferente** pro que é, na prática, a **mesma ocorrência** — os sinks contariam duas vezes o que devia ser uma. O `eventId` precisa nascer **uma única vez, no chamador**, antes do loop de retry, e viajar idêntico em todas as tentativas — exatamente a mesma garantia que `(session_id, player_id, event)` já dava, só que granular por ocorrência em vez de por tipo.

Por isso: o `event_id` é um **campo novo do request**, gerado pelo **data plane** (Unity), não pelo session. O session só repassa — vira um passthrough puro no envelope, igual a `mode_id` já é hoje.

## Comportamento

- WHEN o server do mundo chama `ReportParticipationEvent(..., event_id)` → THE SYSTEM SHALL incluir o `event_id` recebido, **sem gerar nem validar**, no envelope publicado no fan-out (`participation_event`).
- IF `event_id` vier vazio (data plane antigo, antes da spec 004 do Unity) → THE SYSTEM SHALL publicar mesmo assim (campo aditivo, opcional) — os sinks decidem como tratar vazio (ver specs irmãs; provavelmente cai de volta no comportamento de hoje, um dedupe menos fino).
- THE SYSTEM SHALL **não** usar `event_id` para nenhuma lógica própria do session (a idempotência dos efeitos do session — despawn de slot, fechamento do mundo — já é naturalmente idempotente, ver `session/013`). O campo existe só pros sinks.

## Contratos de Interface

### Protobuf (`session/v1/session.proto`)

- `ReportParticipationEventRequest` += `string event_id = 6` (índices 1–5 ocupados: `session_id`/`player_id`/`role`/`event`/`outcome`).
- Regenerar com `buf` (nunca `sed`); conferir o índice livre real antes de gerar.

### Ports (domínio)

- `ParticipationEvent` += `EventID string`.

### Erros de domínio

- Nenhum novo.

## Modelo de Dados

Sem mudança de schema no session (não persiste eventos de participação, só publica). O `EventID` passa pelo envelope (`publisher/sns_publisher.go`, `participationEnvelope` += `EventID string`) sem tocar nenhuma tabela deste serviço.

## Fora de escopo

- **Geração do `event_id`** — responsabilidade do data plane (Unity), não desta spec. Ver `unity-host-world-dropin-data-plane/docs/specs/004-boss-combat-lifecycle`.
- **Consumo/idempotência por `event_id`** nos sinks — `achievements/012`, `analytics/007` (specs irmãs).
- **Validação de formato do `event_id`** (UUID bem-formado etc.) — o session não valida, é opaco pra ele.

## Critérios de Aceite

- [ ] `ReportParticipationEvent` com `event_id` preenchido → aparece idêntico no envelope publicado.
- [ ] `ReportParticipationEvent` sem `event_id` (vazio) → publica igual, campo vazio no envelope (sem erro).
- [ ] Nenhuma mudança observável no comportamento de fechamento/slot (`HOST_KILLED`/`DIED`/`LEFT`) — o `event_id` não entra em nenhuma decisão do session.
- [ ] Testes com `-race`; `host_world_dropin` (specs 011–013) sem regressão.
