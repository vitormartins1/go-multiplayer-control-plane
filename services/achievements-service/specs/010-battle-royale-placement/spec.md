# Spec: 010-battle-royale-placement

**Serviço:** achievements-service
**Status:** Rascunho (aguardando aprovação)
**Data:** 2026-07-07
**ADR:** 0023 (perfil de multiplayer por jogo) — arquétipo `battle_royale`
**Roadmap:** `docs/game-models/battle_royale-roadmap.md` — fatia **BR3** (sinks), parte achievements
**Depende de:** BR2 (`session/015` — envelope `kind=placement` no tópico `match-results`) · reusa o dispatch por `kind` e o `ApplyMatchResult` idempotente (specs `007`/`009`, ADR-0020)

## Contexto

BR2 fez o session-service publicar o resultado do `battle_royale` como um envelope `kind=placement` no tópico `match-results` (colocação ordinal por participante). Hoje o consumer de achievements **descarta** esse kind (o `makeHandler` só trata `win_loss_draw` e `participation_event`; kind desconhecido → no-op). Esta fatia faz o achievements **reagir** ao `placement`, concedendo conquistas de battle_royale.

O padrão já foi provado 2× (queue_arena `007`, host_world_dropin `009`): o consumer despacha por `kind`, traduz o envelope em **event types** de gameplay e delega ao repo a aplicação idempotente por `matchID`. Esta fatia adiciona o **terceiro ramo** (`placement`) — mudança aditiva, sem migration (reusa `processed_matches`).

**Fronteira:** o achievements não valida a corrida nem recomputa colocações — o resultado do server é autoritativo (BR2 já validou estruturalmente). Aqui só traduzimos colocação → event types e stats.

## Comportamento

### Consumir o envelope `placement`

- WHEN o consumer recebe uma mensagem com `kind=placement` → THE SYSTEM SHALL desserializar o envelope (`matchId`, `gameId`, `entries[]`), traduzir cada entrada em **event types** de gameplay (abaixo) e aplicar de forma **idempotente por `matchId`** (reusa `repo.ApplyMatchResult`/`processed_matches`).
- IF a partida já foi processada (`matchId` visto) → no-op (`dup=true`), mensagem deletada normalmente (SQS at-least-once).
- IF o envelope `placement` for malformado (JSON inválido) → erro → mensagem **não** deletada → retry/DLQ (mesmo contrato dos outros kinds).
- THE SYSTEM SHALL manter `win_loss_draw` e `participation_event` **sem regressão**.

### Tradução colocação → event types (política pura do sink)

Espelha `ParticipationEventTypes` (função pura, testável, em `domain/`). Por entrada, a partir da `placement`:

| Condição | Event type emitido |
|---|---|
| sempre (participou) | `battle_royale_played` |
| `placement == 1` | `battle_royale_win` |
| `placement <= TopN` (limiar do sink) | `battle_royale_top_n` |

- **Limiar Top-N:** constante do sink `battle_royale_top_n_threshold = 10` (decisão técnica; ver §Decisões). `placement == 1` também satisfaz `<= 10`, então o 1º lugar acumula `played` + `win` + `top_n`.
- Os event types devem casar com o `event_type` cadastrado no catálogo (o operador cria a conquista "Vitória Battle Royale" com `eventType = "battle_royale_win"`, etc.). Colocação sem event type associado no catálogo → nenhum progresso (comportamento normal).

### Sobrevivência e kills como stats (reuso do critério por-métrica A1)

- WHEN traduz uma entrada → THE SYSTEM SHALL expor `survivedMs` como o stat **`survived_ms`** (injetado no mapa `stats` da entrada, sem sobrescrever se já presente) e repassar os demais `stats` (ex.: `kills`) intactos.
- Assim uma conquista por-stat (`StatKey="survived_ms"` ou `StatKey="kills"`, modos `cumulative`/`per_match` da spec `008`) funciona no battle_royale **sem código novo** — o caminho de stats do `ApplyMatchResult` já cobre.

## Contratos de Interface

### Consumer (`cmd/consumer/main.go`)

- `makeHandler` ganha o ramo `case head.Kind == "placement": return handlePlacement(...)`.
- `handlePlacement` desserializa `{ matchId, gameId, entries[]{ playerId, teamId, placement, survivedMs, stats } }` e chama `svc.ApplyPlacementResult(ctx, gameID, matchID, entries)`.

### Domínio (`domain/`)

- `PlacementEntry{ PlayerID string; Placement int; SurvivedMs int64; Stats map[string]int64 }` (tipo de entrada do sink).
- `PlacementEventTypes(placement int) []string` — função pura (política), espelha `ParticipationEventTypes`.
- consts `EventBattleRoyalePlayed/Win/TopN` e `BattleRoyaleTopNThreshold = 10`; const stat `StatSurvivedMs = "survived_ms"`.

### Serviço (`service/`)

- `ApplyPlacementResult(ctx, gameID, matchID string, entries []domain.PlacementEntry) (dup bool, err error)` — monta `[]PlayerMatchInput` (event types via `PlacementEventTypes`, stats com `survived_ms` injetado) e delega a `repo.ApplyMatchResult` (idempotente por `matchID`). **Reusa** o método de repo existente — sem SQL novo.

### Sem migration, sem RPC, sem store novo

Reusa `processed_matches` e o path `ApplyMatchResult`. Sem coluna nova.

## Casos de Teste Obrigatórios

- [ ] (domain) `PlacementEventTypes(1)` → `[played, win, top_n]`; `PlacementEventTypes(7)` → `[played, top_n]`; `PlacementEventTypes(50)` → `[played]`
- [ ] (service) `ApplyPlacementResult` monta event types por entrada e injeta `survived_ms` nos stats; delega ao repo (mock) com idempotência por `matchID`; `dup` propagado
- [ ] (service) `survived_ms` não sobrescreve um `survived_ms` já presente em `stats` (defensivo)
- [ ] (consumer) envelope `kind=placement` → `handlePlacement` → `ApplyPlacementResult` com as entradas certas; `win_loss_draw`/`participation_event` sem regressão; malformado → erro (DLQ)

## Fora do escopo

- **Catálogo pré-semeado** de conquistas battle_royale (o operador cria via CRUD `006`; podemos semear no seed de dev, mas não é requisito da fatia).
- **Leaderboard/analytics** de colocação — fatias irmãs da BR3 (`leaderboard/005`, `analytics/005`).
- **Console** (surfacing) — BR4.
- **Colocação provisória / tracking ao vivo** — data plane / follow-up (decisões #1/#2 do roadmap).
