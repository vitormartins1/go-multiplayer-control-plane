# Spec: 003-outcome-aggregation

**Serviço:** analytics-service
**Status:** Done
**Data:** 2026-07-06
**ADR:** 0023 (perfil de multiplayer) — fatia **"colher: analytics"** (analytics consome o outcome polimórfico)

## Contexto

O fan-out do resultado virou **v2** (spec `session/009`, ADR-0023): o envelope ganhou `kind` e, por jogador, `outcome` (`"win"|"loss"|"draw"`), `teamId` e `stats`, **mantendo** `won`/`score` por retrocompat. Achievements (`007`) e leaderboard (`003`) já migraram para o `outcome`. O **analytics** é o **último** dos 3 consumidores do fan-out ainda no envelope v1: o `consumer` lê só `{playerId, won, score}` e o rollup conta `wins` a partir de `won`.

Consequência: **o empate é invisível no analytics**. Como `won = (outcome == "win")`, um `draw` chega com `won=false` e é contado como não-vitória — indistinguível de uma derrota. O painel não consegue mostrar empates, e "derrotas" e "empates" ficam colapsados.

Esta fatia faz o analytics **ler o campo `outcome`** e agregar **win/loss/draw** separadamente por jogo. Fecha a paridade dos 3 consumidores do fan-out com o outcome rico. Escopo restrito ao **desfecho** (win/loss/draw); a agregação de **`stats`** (chaves arbitrárias como kills/assists — schema flexível) fica como **follow-up** (ver Fora de escopo).

## Comportamento

### Ingerir o outcome (win/loss/draw), não só `won`

- WHEN o consumidor recebe uma mensagem do fan-out → THE SYSTEM SHALL ler, por jogador, o campo **`outcome`** (`"win"|"loss"|"draw"`) do envelope.
- IF `outcome` estiver ausente/vazio (envelope v1 legado) → THE SYSTEM SHALL derivá-lo de `won` (`won → "win"`, senão `"loss"`) — defesa em profundidade; o produtor atual sempre envia `outcome`.
- THE SYSTEM SHALL manter a leitura de `score` e `playerId` inalterada.

### Agregar win/loss/draw por jogo

- THE SYSTEM SHALL manter, por jogo, contadores separados de **vitórias, derrotas e empates**, incrementados por jogador conforme o `outcome` da partida.
- THE SYSTEM SHALL preservar a **idempotência por `matchID`** (spec `001`): reprocessar uma partida (SQS at-least-once) não recontabiliza desfechos. Marca + agregação na mesma transação — inalterado.
- THE SYSTEM SHALL manter `matches_total`, `participations`, `score_sum` (→ `avgScore` no read), jogadores únicos e a série diária **inalterados**.
- A soma `wins + losses + draws` SHALL igualar `participations` (todo resultado de jogador tem exatamente um desfecho).

### Expor no read

- THE SYSTEM SHALL expor, no `Overview` (cross-game) e no `GameAnalytics` (por jogo), os campos **`losses`** e **`draws`** ao lado de `wins`, no rollup de cada jogo.
- O contrato HTTP existente é **aditivo**: `wins` permanece; `losses`/`draws` entram como campos novos. Nenhum campo removido/renomeado (compat com admin-bff/admin-ui atuais).

## Contratos de Interface

### Ports (domínio)

- **`domain.PlayerResult`** ganha `Outcome string` (`"win"|"loss"|"draw"`). `Won bool` permanece (derivado/compat) ou é substituído pelo `Outcome` — decisão do plan; o importante é o agregador passar a discriminar os três desfechos.
- **`domain.GameStat`** ganha `Losses int64` e `Draws int64` (além de `Wins`).
- `MatchRepository.ApplyMatch/Overview/GameAnalytics` — assinaturas **inalteradas**; passam a povoar/ler os novos contadores.

### HTTP (transport)

- O JSON do overview/detalhe ganha `losses` e `draws` por jogo (aditivo). `wins`, `avgScore`, `matches`, `uniquePlayers` inalterados.

## Modelo de Dados

- **Migration `002`**: `ALTER TABLE game_rollup ADD COLUMN losses BIGINT NOT NULL DEFAULT 0`, idem `draws`. `wins` mantido. Sem tabela nova.
- `ApplyMatch`: além de `wins`, incrementa `losses`/`draws` conforme o `outcome`; UPSERT dos 3 contadores.
- ⚠️ Migrations só rodam em **volume novo** no compose local (gotcha conhecido): recriar `analytics-postgres` (`docker compose ... rm -sfv analytics-service analytics-postgres`) ao aplicar a `002`.

## Fora de escopo (follow-up explícito)

- **Agregação de `stats`** (kills/assists/…): chaves arbitrárias declaradas em `scoringStats` do perfil → precisa de schema flexível (ex.: `game_stat_totals(game_id, stat_key, total)`) + read + UI. Fatia própria ("stats por partida no analytics").
- **Agregação por `teamId`** (win rate por time): depende de "colher por time"; fatia própria.
- **Surfacing no admin-ui** (mostrar empates/derrotas nos StatCards): increment de UI; pode vir junto ou como follow-up curto (admin-bff repassa o JSON aditivo sem mudança).

## Critérios de Aceite

- [ ] Uma partida com win/loss/draw incrementa os três contadores certos; `wins+losses+draws == participations`.
- [ ] Empate deixa de ser contado como derrota (draw distinto de loss).
- [ ] Reprocessar a mesma `matchID` é no-op (idempotência preservada).
- [ ] Envelope sem `outcome` (v1) cai no fallback por `won` sem quebrar.
- [ ] `Overview`/`GameAnalytics` devolvem `losses`/`draws`; JSON aditivo.
- [ ] `matches`/`participations`/`avgScore`/únicos/diária inalterados.
- [ ] Testes ≥ 80% no repositório e no parse do consumidor; `-race`.
