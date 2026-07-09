# Spec: 012-participation-event-id

**Serviço:** achievements-service
**Status:** Done (2026-07-09)
**Data:** 2026-07-09
**ADR:** 0023 — **arquétipo `host_world_dropin`, follow-up #4** (lado achievements: idempotência por `eventId`)
**Roadmap:** `docs/game-models/host_world_dropin-follow-ups.md` → item 4
**Depende de:** `session-service/specs/021-participation-event-id` (o `eventId` precisa existir no envelope)
**Emparelhada com:** `analytics-service/specs/007-participation-event-id-and-outcome` (mesma migração de idempotência, serviço irmão)

## Contexto

`processed_participations` (migration 006, spec 009) dedupe por `(session_id, player_id, event)` — uma ocorrência por tipo de evento por sessão por jogador. Isso conflava corretamente com o cenário de hoje (evento acontece no máximo uma vez por sessão), mas a spec 004 do Unity introduz um boss que **respawna**: o mesmo co-op pode reportar `BOSS_DEFEATED` duas ou mais vezes na mesma sessão, e hoje o segundo report é descartado como "já processado" — a conquista/progresso conta 1 quando devia contar N.

Com `session/021` entregando um `event_id` único por ocorrência (gerado no data plane, ver aquela spec), o achievements troca a chave de idempotência: de `(session_id, player_id, event)` pra `event_id` sozinho — mais simples (chave única em vez de composta) e correto por construção (cada ocorrência tem seu próprio ID).

## Comportamento

- WHEN um `participation_event` chega com `eventId` preenchido → THE SYSTEM SHALL usar `eventId` como chave de idempotência (INSERT em `processed_participations` com `event_id` como chave; `ON CONFLICT DO NOTHING`).
- IF `eventId` vier vazio (data plane antigo, antes da spec 004 do Unity) → THE SYSTEM SHALL cair no comportamento **anterior**: dedupe por `(session_id, player_id, event)` (fallback, não regressão) — sintetizar um `event_id` determinístico a partir desses três campos quando vazio, preservando o comportamento de hoje pra quem ainda não manda o campo.
- THE SYSTEM SHALL manter o resto do fluxo (`ParticipationEventTypes`, `findByEventType`, `recordProgress`) **inalterado** — só a chave de dedupe muda, não a lógica de conquista.
- **Dados existentes:** a migration preserva as linhas já processadas, sintetizando `event_id = session_id || ':' || player_id || ':' || event` pra elas (mesmo fallback determinístico acima) — não há re-processamento nem perda de histórico.

## Contratos de Interface

### Consumer (`cmd/consumer/main.go`)

- `handleParticipation`: o `struct` de parsing do envelope ganha `EventID string \`json:"eventId"\``.
- `svc.ApplyParticipationEvent` ganha o parâmetro `eventID string`.

### Domínio / Service

- `ApplyParticipationEvent(ctx, gameID, sessionID, playerID, event string, eventTypes []string, at time.Time)` → ganha `eventID string` (posição a decidir no plan; provavelmente logo após `event`).

### Erros de domínio

- Nenhum novo.

## Modelo de Dados

**Migration 007** (`007_participation_event_id.sql`): `processed_participations` ganha `event_id TEXT`; backfill determinístico nas linhas existentes (`session_id || ':' || player_id || ':' || event`); troca a PK composta por `PRIMARY KEY (event_id)`. Aditiva e não-destrutiva — nenhuma linha é apagada, o histórico permanece consultável.

## Fora de escopo

- **Geração do `eventId`** — vem do `session/021`/Unity, não daqui.
- **Critério de conquista por stat** (follow-up #6) — outro follow-up, sem relação direta.
- **Backfill "de verdade" pras linhas antigas** (recalcular o que teria sido diferente se elas já tivessem `eventId` desde o início) — não aplicável; o fallback determinístico só preserva o comportamento de dedupe que já existia, não reprocessa progresso.

## Critérios de Aceite

- [x] `participation_event` com `eventId` → dedupe por `eventId`; reenviar o **mesmo** `eventId` (retry) → no-op idempotente, igual a hoje.
- [x] Dois `BOSS_DEFEATED` do mesmo `(session,player)` com `eventId` **diferentes** → contam **duas vezes** (o cenário que este follow-up destrava).
- [x] `participation_event` sem `eventId` (vazio) → cai no fallback determinístico, comportamento idêntico ao pré-migration (uma vez por `(session,player,event)`).
- [x] Migration não apaga/reprocessa dados existentes; `processed_participations` histórico consultável igual.
- [x] Testes com `-race`, ≥80% em `service/`/`repository/` (o que já é medido hoje).
