# Plan: 004-participation-analytics

**Serviço:** analytics-service · **Spec:** `spec.md` (Aprovada 2026-07-07) · **ADR:** 0023 (H2, sink)
**Depende de:** `session/013` (envelope `participation_event`) + specs 001–003 (rollup/idempotência reusados).

Como o analytics **despacha por `kind`** e agrega participações por papel/evento. Fatia **aditiva** — o caminho `win_loss_draw` (`ApplyMatch`) não regride.

## Decisões técnicas

### D1 — dispatch por `kind` no consumer (`internal/consumer`)

O `handleBatch` passa a **espiar o `kind`** antes de submeter:

- `participation_event` → `parseParticipation` (novo) → `pool.SubmitParticipation`.
- `win_loss_draw` **ou** ausente-com-`matchId` → `parse` (atual) → `pool.Submit`.
- desconhecido → **deleta** a mensagem (descarta; não é nosso) e segue — não vira partida fantasma nem trava a fila.
- malformado (JSON inválido) → DLQ (comportamento atual).

Um `peekKind(body) (kind, matchID string)` pequeno evita desserializar duas vezes o corpo inteiro.

### D2 — `Ingestor`/pool polimórficos

`domain.Ingestor` ganha `SubmitParticipation(ctx, ParticipationEvent) <-chan error`. O `job` do pool vira polimórfico:

```go
type job struct {
    match         domain.MatchResult
    participation *domain.ParticipationEvent // não-nil => caminho de participação
    done          chan error
}
```

`process`: `if j.participation != nil { repo.ApplyParticipation(...) } else { repo.ApplyMatch(j.match) }`. Mesmos workers, mesma contrapressão, mesmo fan-in do consumer (delete só no sucesso).

### D3 — domínio

- `ParticipationEvent{GameID, SessionID, PlayerID, Role, Event, Outcome string}`.
- `ParticipationStat{Role, Event string; Count int64}`.
- `GameAnalytics` ganha `RoleParticipations []ParticipationStat` (nome distinto do `Participations int64` já existente, que é a contagem do queue_arena).
- `MatchRepository.ApplyParticipation(ctx, ParticipationEvent) (dup bool, err error)`.

### D4 — repo + migration

- **Migration `003_participation_rollup.sql`**: `processed_participations` (PK `(session_id, player_id, event)`) + `game_participation_rollup` (PK `(game_id, role, event)`, `count BIGINT`).
- `ApplyParticipation`: tx única — marca `processed_participations` (`ON CONFLICT DO NOTHING`; `RowsAffected()==0` → `dup=true`, commit) senão `INSERT ... game_participation_rollup ... ON CONFLICT (game_id, role, event) DO UPDATE SET count = count + 1`; commit. Agrega **todos** os eventos por `(role, event)` (sem `outcome` — decisão da spec).
- `GameAnalytics`: query aditiva em `game_participation_rollup` (`WHERE game_id=$1 ORDER BY role, event`) → `RoleParticipations`. `Overview` inalterado.

### D5 — transport (http)

- O handler que serializa `GameAnalytics` inclui `roleParticipations` (aditivo no JSON). `queue_arena` → lista vazia. Teste do mapeamento.

### D6 — observabilidade

- Reusa RED do pool/consumer. `participations_ingested_total{event}` opcional → **adiado para H3** (funil no dashboard); registrado como follow-up, não bloqueia.

## Arquivos a tocar

| Arquivo | Mudança |
|---|---|
| `internal/domain/analytics.go` | `ParticipationEvent`, `ParticipationStat`, `RoleParticipations`, `ApplyParticipation` na Port, `SubmitParticipation` no `Ingestor` |
| `internal/domain/participation_test.go` (novo) | se houver lógica pura (mapeamento é trivial; talvez dispensável) |
| `internal/service/pool.go` | `job` polimórfico + `SubmitParticipation` + `process` |
| `internal/service/pool_test.go` | caminho de participação (mock repo) |
| `internal/consumer/consumer.go` | dispatch por `kind` + `parseParticipation` |
| `internal/consumer/consumer_test.go` | roteamento por kind |
| `internal/repository/postgres_analytics.go` | `ApplyParticipation` + `RoleParticipations` no read |
| `internal/repository/postgres_analytics_test.go` | integração (`//go:build integration`) |
| `internal/transport/http_handler.go` | `roleParticipations` no JSON do detalhe |
| `internal/transport/http_handler_test.go` | mapeamento aditivo |
| `migrations/003_participation_rollup.sql` (novo) | tabelas |

## Testes

- **Baseline** `go test ./services/analytics-service/... -race` verde antes.
- `service` (mock repo): `SubmitParticipation` → `ApplyParticipation` chamado; `Submit` → `ApplyMatch` (inalterado).
- `consumer`: `kind=participation_event` → caminho de participação; `win_loss_draw`/v1 → match; desconhecido → descarta (delete, sem submit).
- `repository` (integration, gated): reprocessar `(session, player, event)` → dup sem dobrar; `(role, event)` distintos acumulam; `GameAnalytics.RoleParticipations` reflete.
- `transport`: `roleParticipations` no JSON; vazio no queue_arena.
- Cobertura ≥80% em `service/`/`domain/`.

## Sequência de tasks (preview)

1. Baseline verde.
2. Domínio: tipos + Ports.
3. Migration `003` + repo `ApplyParticipation` + `RoleParticipations` (+ integração).
4. Pool polimórfico + `SubmitParticipation` (+ testes).
5. Consumer dispatch por `kind` (+ testes).
6. Transport: `roleParticipations` no JSON (+ teste).
7. `go test ./... -race` verde; lint. Atualizar README (H2 sink analytics). Consolidar follow-ups no doc de evolução.

## Riscos / gotchas

- **Colisão de nome:** `Participations int64` (queue_arena) já existe em `GameStat`; o novo é `RoleParticipations []ParticipationStat`.
- **Não regredir `win_loss_draw`:** o dispatch mantém o envelope v1 (sem kind, com matchId) no `ApplyMatch`.
- **`kind` desconhecido é descarte com delete** (não DLQ, não fantasma).
- **Integração gated** (Docker); o verde padrão cobre domínio+service+consumer+transport.
