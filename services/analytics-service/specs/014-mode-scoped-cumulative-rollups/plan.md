# Plan — 014 mode_id nos rollups cumulativos

## Decisões técnicas

1. **Mesmo padrão de migration já usado na 007** (`ADD COLUMN` + `DROP
   CONSTRAINT <table>_pkey` + `ADD PRIMARY KEY`) — precedente direto no
   próprio projeto, zero ambiguidade de sintaxe/nome de constraint (Postgres
   nomeia PK inline como `<table>_pkey` por padrão, confirmado nas tabelas
   candidatas — todas criadas com `PRIMARY KEY` inline no `CREATE TABLE`).
2. **Leitura por agregação (`SUM`/`COUNT(DISTINCT)`) com `AND ($N='' OR
   mode_id=$N)`** — mesmo padrão consolidado nas specs 010/011/012/013
   (`dailySeries`, `Stats`, `SessionHealth`). `QueryRow` com `Scan` direto
   (usado hoje pra `game_rollup`/`game_placement_rollup`/
   `game_mission_rollup`, uma linha por jogo) não serve mais — pode haver
   N linhas (uma por modo) — vira `SELECT SUM(...) ... WHERE ...
   GROUP BY game_id` (uma linha de resultado, `COALESCE` pro caso sem
   nenhuma linha).
3. **`game_daily` passa a reusar `dailySeries`** (já existe, spec 010) —
   uma vez que o shape bate (`game_id, mode_id, day, matches`), manter uma
   query bespoke separada seria duplicação sem motivo.
4. **`Overview()` ganha `GROUP BY r.game_id`** — achado crítico da revisão:
   sem isso, jogos com >1 modo duplicariam no board cross-game (regressão
   nova, não bug preexistente — motivo pra tratar como parte OBRIGATÓRIA
   desta spec, não como possível follow-up).
5. **As duas subqueries de `game_player_seen` (`Overview` e
   `GameAnalytics`) trocam `count(*)` por `count(DISTINCT player_id)`** —
   achado adicional: uma vez que `game_player_seen` ganhe `mode_id`, um
   jogador que jogou 2 modos vira 2 linhas; sem o `DISTINCT`, contaria
   duplicado no total "todos os modos".

## Arquivos por camada

- `migrations/011_mode_scoped_cumulative_rollups.sql` (novo): as 5
  `ALTER TABLE` do spec.md.
- `internal/repository/postgres_analytics.go`:
  - `ApplyMatch`: `game_rollup`/`game_daily`/`game_player_seen` passam a
    gravar `m.ModeID` (já disponível desde a spec 012); `ON CONFLICT
    (game_id, mode_id)` (ou `(game_id, mode_id, day)`/`(game_id, mode_id,
    player_id)`, conforme a tabela).
  - `ApplyPlacement`: `game_placement_rollup` grava `pr.ModeID`.
  - `ApplyCollective`: `game_mission_rollup` grava `cr.ModeID`.
  - `GameAnalytics`:
    - `game_rollup`: `QueryRow` vira `SELECT COALESCE(SUM(...),0) ...
      WHERE game_id=$1 AND ($2='' OR mode_id=$2)`.
    - `game_daily`: troca a query bespoke por `r.dailySeries(ctx,
      "game_daily", "matches", gameID, days, modeID)`.
    - `game_placement_rollup`/`game_mission_rollup`: mesma troca de
      `QueryRow` simples por `SUM(...) WHERE ... GROUP BY` (ou só agregação
      sem GROUP BY já que filtra por `game_id` fixo — uma linha de saída).
    - As duas subqueries de `game_player_seen`: `count(*)` ->
      `count(DISTINCT player_id)` + `AND ($N='' OR mode_id=$N)`.
  - `Overview()`: query principal ganha `GROUP BY r.game_id` + troca pra
    `SUM(...)` em cada coluna; subquery de `game_player_seen` ganha
    `DISTINCT`; a query global de `count(DISTINCT player_id) FROM
    game_player_seen` (sem filtro por jogo) já está correta de fábrica
    (dedupe cross-jogo-e-modo é o comportamento certo pro total geral).
- Testes: `postgres_analytics_test.go` (integration) — regressão zero sem
  filtro; filtro isola por modo nos 3 rollups + daily + unique players;
  `Overview` não duplica jogo com 2+ modos; jogador em 2 modos não conta
  duplicado no total global.

## Ordem de implementação

Migration → repository (writers: `ApplyMatch`/`ApplyPlacement`/
`ApplyCollective`) → repository (readers: `GameAnalytics`/`Overview`) →
testes → build/test completo → validação ao vivo (jogo real com 2+ modos
queue_arena, comparar `summary`/`outcomes-donut` filtrado vs não-filtrado;
`Overview` continua 1 linha por jogo) → admin-ui (spec-irmã 066, remove o
aviso obsoleto).
