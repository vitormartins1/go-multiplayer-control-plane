# Plan — 009 quebra por modo nas participações

## Decisões técnicas

1. **Backward compat via bucket `''`** — mesmo padrão da migration 007
   (`outcome`). Dado antigo (sem `mode_id` no envelope, ou processado antes
   desta mudança) grava/lê com `mode_id=''`; a UI mostra isso como "sem
   modo". Não há backfill retroativo (mesma decisão já tomada pra outcome —
   não reconstituível sem os eventos originais).
2. **Chave primária** de `game_participation_rollup` passa de
   `(game_id, role, event, outcome)` para
   `(game_id, mode_id, role, event, outcome)` — mesma mecânica de ALTER da
   migration 006 (drop constraint, add column, add PK nova).
3. **`processed_participations`** (idempotência) não muda — a chave já é
   `event_id`, independente de mode_id.
4. **UI**: o filtro de modo só considera os modos com `archetype ===
   "host_world_dropin"` do jogo (já vêm de `game.multiplayerModes`,
   ADR-0024) — não precisa de chamada nova à API. `modeId` sem modo
   correspondente na lista (ex.: `""` ou um modo já deletado) cai num item
   fixo "sem modo" no seletor.
5. **Sem mudança em `session-service`** — o `modeId` já é publicado no
   envelope (`sns_publisher.go`); é só destravar a leitura no
   analytics-service.

## Arquivos por camada

**analytics-service:**
- `migrations/007_participation_mode_id.sql` (novo) — `ALTER TABLE
  game_participation_rollup ADD COLUMN mode_id TEXT NOT NULL DEFAULT ''`;
  drop da PK atual; `ADD PRIMARY KEY (game_id, mode_id, role, event,
  outcome)`.
- `internal/consumer/consumer.go` — `parseParticipation`: struct anônimo do
  envelope ganha `ModeID string \`json:"modeId"\``; passa pro
  `domain.ParticipationEvent`.
- `internal/domain/analytics.go` — `ParticipationEvent.ModeID string` e
  `ParticipationStat.ModeID string`.
- `internal/repository/postgres_analytics.go`:
  - `ApplyParticipation`: INSERT/ON CONFLICT do rollup passa a incluir
    `mode_id` na lista de colunas e no conflict target.
  - Query de leitura (dentro de `GameAnalytics`, ~linha 307): `SELECT
    mode_id, role, event, outcome, count ...` e `Scan(&p.ModeID, &p.Role,
    ...)`.
- `internal/transport/http_handler.go` — `participationStatDTO` ganha
  `ModeID string \`json:"modeId"\``, propagado no mapeamento pra DTO.
- Testes: `consumer_test.go` (parseParticipation com/sem modeId no JSON),
  `game_service_test.go`/equivalente (ApplyParticipation com 2 modos
  diferentes não somam entre si), `postgres_analytics_test.go` (round-trip
  da coluna nova + migration 007 na lista de init scripts do teste),
  `http_handler_test.go` (DTO inclui modeId).

**admin-bff:**
- `internal/domain/analytics.go` (ou onde estiver `ParticipationStat`) —
  `ModeID string \`json:"modeId"\``. Passthrough puro, sem lógica nova.
- Teste correspondente do passthrough (se existir um específico pra
  roleParticipations).

**admin-ui:**
- `src/lib/api.ts` — `ParticipationStat.modeId: string`.
- `src/pages/GameAnalytics.tsx`:
  - Recebe (ou busca) `game.multiplayerModes` — checar se `GameDetail.tsx`
    já passa `game` inteiro pro componente ou só `gameId`; se só `gameId`,
    busca via `api.listMultiplayerModes(gameId)` (mesmo padrão do
    `GamePipeline.tsx`).
  - Deriva a lista de modos host_world_dropin presentes nas
    `roleParticipations` (não só no cadastro — um modo deletado depois de
    gerar dado ainda deve aparecer no seletor, senão o dado "some" da UI).
  - `SegmentedControl` ou `Select` com "Todos os modos" (default) + um item
    por `modeId` distinto (nome de exibição do cadastro se existir, senão o
    próprio `modeId`, ou "sem modo" se `""`).
  - Filtra `participations` pelo modo selecionado antes de chamar
    `participationByRole` e antes de renderizar a tabela.

## Ordem de implementação

Backend (analytics-service) → admin-bff (passthrough) → admin-ui. Cada
camada só faz sentido testar depois que a de baixo já propaga o campo —
sequencial, não paralelo.
