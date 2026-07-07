# Spec: 004-cumulative-boards

**Serviço:** leaderboard-service (+ consumidor do fan-out)
**Status:** Draft
**Data:** 2026-07-07
**ADR:** 0023 (resultado polimórfico) — fatia **A2** do roadmap `queue_arena`; relaciona ADR-0020 (fan-out).

## Contexto

A fatia 003 deu N rankings **highscore** por jogo (`score` + um por stat, melhor de uma partida, `ZADD GT`). Esta fatia adiciona um segundo **modo de agregação**: **cumulative** (`ZINCRBY`) — rankings que somam ao longo das partidas: **vitórias** (`wins`) e **totais de stat** (`<stat>_total`, ex.: `kills_total`). Os dois modos convivem no mesmo esquema de keys; a leitura (`?board=…`) já serve qualquer board sem mudança.

**Gotcha central:** `ZINCRBY` **não** é idempotente de graça (diferente do `ZADD GT` e da tx do achievements). Como o SQS entrega at-least-once, reprocessar a mesma partida dobraria os totais. A idempotência é resolvida por uma **guarda por matchID** aplicada junto dos incrementos, **atômica** (script Lua: `SADD` do matchID + os `ZINCRBY` num só passo indivisível).

## Comportamento

### Boards cumulativos (ZINCRBY)

- WHEN o fan-out entrega uma partida → THE SYSTEM SHALL, por jogador: somar **+1** ao board `wins` se `outcome == "win"`, e somar o valor de cada `stat` ao board `<stat>_total`.
- THE SYSTEM SHALL aplicar todas as parcelas cumulativas de uma partida numa **única operação atômica** guardada por `matchID`: se a partida já foi aplicada, é **no-op** (`dup=true`); senão marca e soma.
- THE SYSTEM SHALL manter os boards **highscore** (`score` + cada stat) inalterados (aplicados fora da guarda, pois `ZADD GT` já é idempotente).
- Parcelas com nome de board inválido (fora de `^[a-z][a-z0-9_]*$`) são descartadas; sem parcelas válidas é no-op.

### Leitura

- WHEN `GET /leaderboard?board=wins` (ou `?board=kills_total`) → THE SYSTEM SHALL devolver o Top do board cumulativo. Sem mudança na API de leitura (a fatia 003 já parametrizou `board`).

## Contratos de Interface

- **Domain:** `BoardWins` const; `TotalBoard(stat) string`; tipo `Increment{Board, PlayerID string; Delta int64}`. `LeaderboardService`/`LeaderboardStore` +`ApplyCumulative(ctx, gameID, matchID string, incrs []Increment) (dup bool, err error)`.
- **Storage (Redis):** `ApplyCumulative` via script Lua (`SADD` no SET `<prefix>:processed:<gameID>` + `ZINCRBY` por parcela). Atômico e idempotente.
- **Service:** valida/filtra boards; delega ao store.
- **Consumer:** lê `outcome`+`stats`; monta as parcelas (`wins`, `<stat>_total`) e chama `ApplyCumulative` uma vez por partida, além dos `Submit` highscore existentes.

## Casos de Teste Obrigatórios

- [ ] (storage, integração) soma entre partidas distintas; reentrega da mesma partida = no-op (não dobra); board cumulativo não afeta o highscore — testcontainers redis
- [ ] (service) parcela com board inválido é descartada; dup propaga
- [ ] (transport) `GET /leaderboard?board=wins` serve o board (via fatia 003, regressão)

## Fora do escopo

- **Razões** (K/D) e métricas derivadas.
- **Seasons/janelas por período** (reset temporal do SET/boards) — backlog; hoje o SET `processed` cresce sem limite (aceitável em dev; TTL/rolling em prod).
- **Cluster Redis:** o script assume nó único (as keys de board vão no ARGV, não como KEYS com hash-tag) — coerente com o setup local.
- **`teamId`** (ranking por time) — evolução.
