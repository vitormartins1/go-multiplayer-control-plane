# Spec: 005-battle-royale-placement

**Serviço:** leaderboard-service
**Status:** Rascunho (aguardando aprovação)
**Data:** 2026-07-07
**ADR:** 0023 (perfil de multiplayer por jogo) — arquétipo `battle_royale`
**Roadmap:** `docs/game-models/battle_royale-roadmap.md` — fatia **BR3** (sinks), parte leaderboard
**Depende de:** BR2 (`session/015` — envelope `kind=placement`) · reusa boards highscore (ZADD GT) + cumulativos (ZINCRBY idempotente por `matchID`) das specs `003`/`004` (ADR-0023 A2)

## Contexto

BR2 publica o resultado do `battle_royale` como `kind=placement`. Hoje o consumer de leaderboard (`cmd/consumer/main.go`) **não despacha por kind** — assume sempre o envelope `win_loss_draw` (`results[]`). Um envelope `placement` cai nesse parse, encontra `results` vazio e vira no-op silencioso (não quebra, mas não rankeia o BR). Esta fatia adiciona o **dispatch por `kind`** e o ramo `placement`, alimentando os boards do battle_royale.

Reusa os dois modos já provados (spec `004`): **highscore** (`ZADD GT`, idempotente por natureza) e **cumulative** (`ZINCRBY` numa chamada atômica idempotente por `matchID` via `ApplyCumulative`). Só o **mapeamento** entrada→boards é novo.

**Fronteira:** o leaderboard não valida colocação — o server é autoritativo (BR2 validou). Aqui só mapeamos colocação/stats → boards.

## Comportamento

### Dispatch por kind (refactor mínimo)

- THE SYSTEM SHALL passar a **despachar por `kind`** no `makeHandler` (hoje implícito `win_loss_draw`): `placement` → novo mapeamento; `win_loss_draw` (ou envelope v1 legado com `matchId` e sem kind) → o mapeamento atual **sem regressão**; kind desconhecido → no-op (descarta).

### Consumir o envelope `placement`

- WHEN o consumer recebe `kind=placement` → THE SYSTEM SHALL, por entrada, alimentar:
  - **highscore** (`ZADD GT`): board **`survived_ms`** com `survivedMs`; e um board highscore por stat presente (ex.: `kills`) — mesmo loop do `win_loss_draw`.
  - **cumulative** (`ZINCRBY`, idempotente por `matchID`): board **`wins`** `+1` se `placement == 1` (reusa o board de vitórias existente); board **`top_n_finishes`** `+1` se `placement <= TopN`; e **`<stat>_total`** por stat presente (ex.: `kills_total`).
- THE SYSTEM SHALL aplicar a parte cumulativa numa **única chamada** `ApplyCumulative(gameID, matchID, incrs)` — idempotente por `matchID` (reprocessar a partida é no-op).
- **Limiar Top-N:** constante do sink `battle_royale_top_n_threshold = 10` (consistente com achievements). `placement == 1` conta como `win` **e** `top_n_finishes`.
- IF envelope malformado → erro → não deleta → retry/DLQ.

### Sem novos nomes livres de board fora do vocabulário

- `wins` já existe (`BoardWins`); `top_n_finishes` é board novo (nome fixo, passa no `boardRE`/`ValidBoard`); `survived_ms` e `<stat>_total` seguem as regras existentes de nome.

## Contratos de Interface

### Consumer (`cmd/consumer/main.go`)

- `makeHandler` passa a espiar `kind` (como achievements/analytics já fazem) e ramifica: `handlePlacement` vs. o atual (extraído para `handleWinLossDraw`, comportamento idêntico).
- `handlePlacement` desserializa `{ matchId, gameId, entries[]{ playerId, teamId, placement, survivedMs, stats } }` e monta os `Submit` (highscore) + `[]domain.Increment` (cumulative), fechando com `svc.ApplyCumulative`.

### Domínio (`domain/`)

- const `BoardTopNFinishes = "top_n_finishes"`; const `BoardSurvivedMs = "survived_ms"`; const `BattleRoyaleTopNThreshold = 10`. Reusa `BoardWins`, `TotalBoard`, `Increment`, `ValidBoard`.

### Sem migration, sem store novo

Redis; reusa `Submit`/`ApplyCumulative`. Sem estrutura nova.

## Casos de Teste Obrigatórios

- [ ] (consumer) `kind=placement`: `placement==1` gera `wins+1`, `top_n_finishes+1`, highscore `survived_ms`, e `kills_total`/highscore `kills` quando há `kills`
- [ ] (consumer) `placement==7` (≤10) gera `top_n_finishes+1` mas **não** `wins`; `placement==50` não gera nenhum dos dois
- [ ] (consumer) cumulative aplicado numa única `ApplyCumulative` idempotente por `matchID` (reprocesso = `dup`, no-op)
- [ ] (consumer) `win_loss_draw` e envelope v1 legado **sem regressão**; kind desconhecido → descarta; malformado → DLQ

## Fora do escopo

- **Board de "melhor colocação"** (menor placement) — exigiria board `LT`/inversão; não é pedido do roadmap (foco em wins/top_n/kills/sobrevivência). Follow-up se surgir.
- **Achievements/analytics** de colocação — fatias irmãs.
- **Console** (surfacing dos boards BR) — BR4.
