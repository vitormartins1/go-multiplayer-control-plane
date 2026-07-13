# Spec: 014-mode-scoped-cumulative-rollups

**Serviço:** analytics-service (fatia de limpeza em admin-ui)
**Status:** Done
**Data:** 2026-07-13
**Parte de:** evolução do Analytics (item D, último da ordem A→B→C→D
decidida com o usuário) — fecha a lacuna que os itens A/B documentaram e
contornaram (spec 012: "fora de escopo"; spec 064: os 5 avisos "ainda soma
todos os modos").

## Contexto

`game_rollup`, `game_daily`, `game_placement_rollup`, `game_mission_rollup`
e `game_player_seen` nunca tiveram `mode_id` — são as tabelas mais antigas
do analytics-service (specs 001/002/004/005), anteriores ao ADR-0024. Todo
o resto (`game_participation_rollup`, as 3 tabelas `*_daily` novas da spec
010, `game_player_stats`, `game_session_rollup`) já tem a dimensão. Efeito:
5 widgets do admin-ui (`summary`, `queue-daily`, `outcomes-donut`,
`placement-distribution`, `mission-distribution`) mostram sempre o total
de TODOS os modos do arquétipo, mesmo com um modo específico selecionado —
hoje comunicado por um aviso (spec 064), não corrigido.

**Achado na revisão desta spec, antes de implementar:** `Overview()`
(painel cross-game) faz `SELECT ... FROM game_rollup r ORDER BY
r.matches_total DESC` **sem `GROUP BY`**. Se `game_rollup` ganhar
`mode_id` na chave primária (uma linha por modo, não mais por jogo), essa
query passaria a devolver **múltiplas linhas por jogo** (uma por modo
jogado) — o board cross-game duplicaria jogos com mais de um modo. Setup
teria que mudar pra `GROUP BY r.game_id` com `SUM(...)`, senão é uma
regressão nova introduzida por esta spec, não um bug preexistente.

**Achado adicional:** a subquery de jogadores únicos
(`SELECT count(*) FROM game_player_seen s WHERE s.game_id = r.game_id`,
usada tanto em `Overview` quanto em `GameAnalytics`) depende de
`game_player_seen` ter NO MÁXIMO uma linha por `(game_id, player_id)` — é
assim que `count(*)` já equivale a "jogadores distintos" hoje. Uma vez que
`game_player_seen` ganhe `mode_id` na chave (pra saber quantos jogadores
distintos jogaram um MODO específico), um jogador que passou por 2 modos
vira 2 linhas — `count(*)` sem filtro passaria a contar duplicado. As duas
subqueries precisam virar `count(DISTINCT player_id)`.

## Comportamento

- WHEN `modeID == ""` (sem filtro, "Todos os modos") -> THE SYSTEM SHALL
  continuar somando TODOS os modos do jogo/arquétipo — mesmo total de hoje,
  regressão zero (validado byte-a-byte contra o comportamento atual).
- WHEN `modeID != ""` -> `Matches`/`Participations`/`Wins`/`Losses`/
  `Draws`/`AvgScore`/`UniquePlayers`/`Daily` (base, queue_arena) e
  `Placement`/`Mission` (battle_royale/coop_mission) SHALL refletir SÓ o
  modo pedido — fecha a lacuna que os 5 widgets da spec 064 avisavam.
- `Overview()` (painel cross-game, sem conceito de modo — sempre "todos os
  modos" de todos os jogos) SHALL continuar devolvendo UMA linha por jogo
  (agregando os modos internamente, `GROUP BY game_id`) — sem essa
  correção, vira regressão (jogos com >1 modo duplicariam no board).
- Dado legado (linhas gravadas antes desta migration) cai no bucket
  `mode_id=''` — mesmo padrão já usado em toda dimensão nova deste projeto,
  soma corretamente no agregado "todos os modos".

## Contratos de Interface

### Schema (migration 011 do analytics-postgres)

Mesmo padrão já usado na migration 007 (`game_participation_rollup`):
`ADD COLUMN` + `DROP CONSTRAINT <table>_pkey` + `ADD PRIMARY KEY` com
`mode_id` na chave.

```sql
ALTER TABLE game_rollup ADD COLUMN mode_id TEXT NOT NULL DEFAULT '';
ALTER TABLE game_rollup DROP CONSTRAINT game_rollup_pkey;
ALTER TABLE game_rollup ADD PRIMARY KEY (game_id, mode_id);

ALTER TABLE game_daily ADD COLUMN mode_id TEXT NOT NULL DEFAULT '';
ALTER TABLE game_daily DROP CONSTRAINT game_daily_pkey;
ALTER TABLE game_daily ADD PRIMARY KEY (game_id, mode_id, day);

ALTER TABLE game_placement_rollup ADD COLUMN mode_id TEXT NOT NULL DEFAULT '';
ALTER TABLE game_placement_rollup DROP CONSTRAINT game_placement_rollup_pkey;
ALTER TABLE game_placement_rollup ADD PRIMARY KEY (game_id, mode_id);

ALTER TABLE game_mission_rollup ADD COLUMN mode_id TEXT NOT NULL DEFAULT '';
ALTER TABLE game_mission_rollup DROP CONSTRAINT game_mission_rollup_pkey;
ALTER TABLE game_mission_rollup ADD PRIMARY KEY (game_id, mode_id);

ALTER TABLE game_player_seen ADD COLUMN mode_id TEXT NOT NULL DEFAULT '';
ALTER TABLE game_player_seen DROP CONSTRAINT game_player_seen_pkey;
ALTER TABLE game_player_seen ADD PRIMARY KEY (game_id, mode_id, player_id);
```

`game_daily` passa a ter o MESMO shape de `game_placement_daily`/
`game_mission_daily`/`game_participation_daily` — a leitura em
`GameAnalytics` pode passar a reaproveitar o helper `dailySeries` já
existente (spec 010), em vez da query bespoke atual.

### Domínio — SEM mudança de tipo

`GameAnalytics`/`Overview`/`GameStat`/`PlacementDistribution`/
`MissionDistribution` já têm os campos certos — só a ORIGEM do dado deixa
de ser sempre-global.

## Critérios de Aceite

1. Sem `?modeId=` -> resposta idêntica à atual em `GameAnalytics` E em
   `Overview` (regressão zero, validado com dado real multi-modo).
2. Jogo `queue_arena` com 2 modos, um selecionado -> `matches`/`wins`/
   `losses`/`draws`/`avgScore`/`uniquePlayers`/`daily` refletem SÓ aquele
   modo.
3. Jogo `battle_royale`/`coop_mission` com 2 modos, um selecionado ->
   `placement`/`mission` refletem SÓ aquele modo.
4. `Overview()` com um jogo de 2+ modos -> continua UMA linha nesse jogo
   (não duplica), com os totais somados entre os modos.
5. `UniquePlayers` (global E por modo) continua correto quando um jogador
   jogou mais de um modo do mesmo jogo (não conta duplicado no total
   global; conta 1 em cada modo que jogou).
6. Dado legado (linhas de antes desta migration, `mode_id=''`) -> segue
   contando no agregado "todos os modos", sem quebra.

## Fora de escopo

- Séries diárias filtradas por `Placement`/`Mission`/`game_rollup` já
  cobertas pelas tabelas `*_daily` — nada muda aí, é só a leitura de
  `game_daily` que passa a reusar o helper comum.
- Qualquer UI nova — a fatia de admin-ui desta spec é só remover o aviso
  que ficou obsoleto (spec 064), não adicionar nada.
