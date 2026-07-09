# Plan: 007-participation-event-id-and-outcome

**Serviço:** analytics-service · **Spec:** `spec.md` (Proposta 2026-07-09) · **ADR:** 0023 (follow-ups #4+#5, lado analytics)

Duas mudanças na mesma tabela-família: `eventId` na idempotência (espelha `achievements/012`) + `outcome` na chave do rollup (novo, só aqui).

## Decisões técnicas

### D1 — `effectiveEventID`, mesmo padrão do achievements

Idêntico a `achievements/012` D1 (função pura, fallback determinístico `session_id:player_id:event` quando `eventId` vazio) — duplicado deliberadamente entre os dois serviços (mesma lógica dos.md de hash de senha em `session/020`/`matchmaking/011`: serviços independentes, função trivial, sem pacote compartilhado).

### D2 — migration única com duas partes independentes

`006_participation_event_id_and_outcome.sql`:

```sql
-- parte 1: eventId (idêntica à achievements/012, tabela própria deste serviço)
ALTER TABLE processed_participations ADD COLUMN event_id TEXT;
UPDATE processed_participations
  SET event_id = session_id || ':' || player_id || ':' || event
  WHERE event_id IS NULL;
ALTER TABLE processed_participations ALTER COLUMN event_id SET NOT NULL;
ALTER TABLE processed_participations DROP CONSTRAINT processed_participations_pkey;
ALTER TABLE processed_participations ADD PRIMARY KEY (event_id);

-- parte 2: outcome no rollup (só aqui, achievements não tem este rollup)
ALTER TABLE game_participation_rollup ADD COLUMN outcome TEXT NOT NULL DEFAULT '';
ALTER TABLE game_participation_rollup DROP CONSTRAINT game_participation_rollup_pkey;
ALTER TABLE game_participation_rollup ADD PRIMARY KEY (game_id, role, event, outcome);
```

Linhas existentes de `game_participation_rollup` ficam com `outcome=''` — não reconstituível (só o total sobreviveu, não os eventos individuais que o compuseram).

### D3 — `ApplyParticipation` grava outcome na chave do rollup

```go
tx.Exec(ctx, `
    INSERT INTO game_participation_rollup (game_id, role, event, outcome, count)
    VALUES ($1, $2, $3, $4, 1)
    ON CONFLICT (game_id, role, event, outcome) DO UPDATE SET count = game_participation_rollup.count + 1`,
    evt.GameID, evt.Role, evt.Event, evt.Outcome)
```

`processed_participations` usa `effectiveEventID(evt.EventID, evt.SessionID, evt.PlayerID, evt.Event)` (D1) na `INSERT ... ON CONFLICT (event_id) DO NOTHING`, mesmo padrão do achievements.

### D4 — leitura (`RoleParticipations`)

Query ganha `outcome` no `SELECT`/`ORDER BY`; `domain.ParticipationStat` ganha o campo `Outcome string`; o scan em `RoleParticipations` (postgres_analytics.go) inclui a nova coluna.

### D5 — consumer

`parseParticipation` (`internal/consumer/consumer.go`): `struct` local ganha `EventID string \`json:"eventId"\``; `domain.ParticipationEvent` (analytics) ganha `EventID string`; `ApplyParticipation` recebe o evento já com o campo populado (sem parâmetro extra na assinatura — `ParticipationEvent` já é passado inteiro, diferente do achievements que usa parâmetros soltos).

## Arquivos a tocar

| Arquivo | Mudança |
|---|---|
| `migrations/006_participation_event_id_and_outcome.sql` (novo) | D2 |
| `internal/domain/analytics.go` | `ParticipationEvent.EventID`; `ParticipationStat.Outcome` |
| `internal/repository/postgres_analytics.go` | `effectiveEventID` (D1); `ApplyParticipation` (D3); `RoleParticipations`/query do `GameAnalytics` (D4) |
| `internal/consumer/consumer.go` | `parseParticipation` lê `eventId` (D5) |
| testes correspondentes | dois `eventId` diferentes contam 2×; `HOST_KILLED` SUCCESS/FAIL separados; `eventId` vazio → fallback |

## Testes

- Baseline verde antes.
- `repository`: `ApplyParticipation` com dois `eventId` diferentes (mesmo session/player/event) → rollup soma 2. Mesmo evento/outcome, `eventId` diferente → linhas de rollup **iguais** em `(game,role,event,outcome)`, `count` soma; `outcome` diferente → linhas **separadas**. `RoleParticipations` devolve `outcome` correto.
- `consumer`: envelope com/sem `eventId` → parseia certo.
- Migration: rodar contra cópia com dados da 003/004/005 (participation + placement + mission convivem no mesmo banco) e confirmar que só as tabelas de participação mudam.
- Cobertura mantida.

## Sequência de tasks (preview)

1. Baseline verde.
2. Migration 006 (D2) — rodar isolada, conferir backfill nas duas tabelas.
3. Domínio: `EventID`/`Outcome` nos structs.
4. Repository: `effectiveEventID` + `ApplyParticipation` com outcome na chave + leitura com outcome.
5. Consumer: parse do `eventId`.
6. Testes (eventId duplicado, outcome separado, fallback vazio).
7. `go test ./... -race -cover` verde; lint.

## Riscos / gotchas

- **Duas mudanças de schema na mesma migration** — mantidas na mesma spec porque tocam a mesma família de tabelas na mesma sessão de trabalho, mas são conceitualmente independentes (poderiam ser revertidas/refeitas em separado se algo der errado).
- **Depende de `session/021`** pro `eventId` (mesma dependência da spec irmã achievements).
- **`outcome=''` nos dados antigos** — não é um bug, é a fronteira honesta do que dá pra migrar sem replay dos eventos originais.
