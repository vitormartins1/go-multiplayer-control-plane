# Plan: 012-participation-event-id

**Serviço:** achievements-service · **Spec:** `spec.md` (Proposta 2026-07-09) · **ADR:** 0023 (follow-up #4, lado achievements)

Troca a chave de idempotência de `(session_id, player_id, event)` pra `event_id`, com fallback determinístico pra quem ainda não manda o campo.

## Decisões técnicas

### D1 — fallback determinístico, não um branch condicional espalhado

Em vez de checar "`eventID` vazio → usa a query antiga" em cada call site, sintetiza o fallback **uma vez**, cedo:

```go
func effectiveEventID(eventID, sessionID, playerID, event string) string {
    if eventID != "" {
        return eventID
    }
    return sessionID + ":" + playerID + ":" + event // mesmo shape da PK antiga
}
```

Chamado logo no início de `ApplyParticipationEvent`; o resto do método (INSERT, `ON CONFLICT`) não sabe se o ID veio do caller ou foi sintetizado. Reduz a mudança a "troca a chave", sem duplicar caminhos.

### D2 — migration com backfill, não truncate

`007_participation_event_id.sql`:

```sql
ALTER TABLE processed_participations ADD COLUMN event_id TEXT;
UPDATE processed_participations
  SET event_id = session_id || ':' || player_id || ':' || event
  WHERE event_id IS NULL;
ALTER TABLE processed_participations ALTER COLUMN event_id SET NOT NULL;
ALTER TABLE processed_participations DROP CONSTRAINT processed_participations_pkey;
ALTER TABLE processed_participations ADD PRIMARY KEY (event_id);
```

O backfill usa o **mesmo formato** de `effectiveEventID` (D1) — dado existente processado sob a chave antiga continua com o mesmo "ID" efetivo, então reenviar (retry tardio) algo que já foi processado antes da migration ainda dedupe certo.

### D3 — assinatura de `ApplyParticipationEvent`

`ApplyParticipationEvent(ctx, gameID, sessionID, playerID, event string, eventTypes []string, at time.Time)` → `ApplyParticipationEvent(ctx, gameID, sessionID, playerID, event, eventID string, eventTypes []string, at time.Time)` (novo parâmetro logo após `event`, mesma vizinhança semântica). `game_id`/`session_id`/`player_id`/`event` continuam gravados na tabela (úteis pra debug/consulta), só a **PK** que muda pra `event_id`.

### D4 — consumer

`handleParticipation` (`cmd/consumer/main.go`): o `struct` local de parsing ganha `EventID string \`json:"eventId"\`` (aceita ausência, vira `""` — `encoding/json` não erra em campo faltante). Chama `svc.ApplyParticipationEvent(..., env.EventID, ...)`.

## Arquivos a tocar

| Arquivo | Mudança |
|---|---|
| `migrations/007_participation_event_id.sql` (novo) | D2 |
| `internal/repository/postgres_achievement.go` | `ApplyParticipationEvent`: `effectiveEventID` (D1) + INSERT usa `event_id` na PK/conflict |
| `internal/service/achievement_service.go` | assinatura repassa `eventID` |
| `internal/domain/achievement.go` (se a Port estiver lá) | assinatura da interface, se declarada |
| `cmd/consumer/main.go` | `handleParticipation` lê `EventID` do envelope, repassa |
| testes correspondentes (`*_test.go`) | casos novos: dois eventos com `eventId` diferentes contam 2×; `eventId` vazio cai no fallback; retry do mesmo `eventId` é no-op |

## Testes

- Baseline verde antes.
- `repository`: `ApplyParticipationEvent` com dois `eventId` diferentes (mesmo session/player/event) → conta as duas vezes. Mesmo `eventId` retry → no-op. `eventId` vazio → dedupe por `(session,player,event)` como antes (duas chamadas vazias com o mesmo trio → só a primeira conta).
- `consumer`: envelope com/sem `eventId` → parseia certo, chama o service com o valor esperado.
- Migration: rodar contra uma cópia com dados da 006 (linhas antigas) e confirmar backfill + PK nova sem perda de linha.
- Cobertura mantida (≥80% já medido nas specs anteriores deste serviço, conferir o baseline atual).

## Sequência de tasks (preview)

1. Baseline verde.
2. Migration 007 (D2) — rodar isolada, conferir backfill.
3. `effectiveEventID` (D1) + `ApplyParticipationEvent` usa `event_id` na PK.
4. Consumer: parse + repasse do `eventId`.
5. Testes (dois eventId diferentes, mesmo eventId, vazio/fallback).
6. `go test ./... -race -cover` verde; lint.

## Riscos / gotchas

- **Depende de `session/021` já estar emitindo `eventId`** — sem isso, todo envelope chega vazio e o fallback (D1) vira o caminho único de fato (comportamento idêntico a hoje, sem regressão, mas também sem o ganho).
- **Migration muda a PK** — não é só `ADD COLUMN`; conferir que nenhum outro código depende da PK antiga (índices, FKs) antes de rodar em qualquer ambiente com dados reais (aqui é só dev/local).
