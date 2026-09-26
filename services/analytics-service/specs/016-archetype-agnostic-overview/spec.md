# Spec: 016-archetype-agnostic-overview

**Serviço:** analytics-service (+ admin-bff passthrough, admin-ui Analytics global)
**Status:** Done
**Data:** 2026-09-26
**Nível:** fatia pequena (correção de agregação + passthrough; sem tabela nova)
**Origem:** auditoria do Analytics na revisão 2026-09 (Fase 3)

## Contexto

O overview cross-game lia só o `game_rollup` (partidas de `queue_arena`) e só o `ApplyMatch` marcava
`game_player_seen`. Resultado: o Folclore Souls (`host_world_dropin`) — e qualquer jogo fora do `queue_arena` —
aparecia no Analytics global com 0 partidas e 0 jogadores únicos, e o "Jogadores únicos" do detalhe por jogo também
ficava em 0 para esses arquétipos.

## Comportamento

- Todos os `Apply*` (partida, participação, colocação, missão) marcam o jogador em `game_player_seen`.
- O overview tem **uma linha por jogo com qualquer sessão fechada ou partida**, com:
  `sessions` (sessões fechadas, `game_session_rollup`), `avgSessionMs`, `abandoned` (`abandoned` + `ttl`),
  `uniquePlayers`, e os campos de `queue_arena` que já existiam (`matches`, `wins`, …). Ordem: sessões, partidas.
- `totals.sessions` novo; `daily` do overview passa a ser **sessões fechadas por dia** (`{day, sessions}`) —
  a série de partidas só existia para `queue_arena`.
- admin-ui: a página global mostra Sessões, Jogadores únicos, Jogos com atividade, Abandono; gráfico "Sessões por
  dia"; tabela Jogo · Sessões · Jogadores únicos · Duração média · Abandono (e "Partidas (arena)" só se algum jogo
  tiver). Vitória/derrota/score saem da visão global (continuam no detalhe por jogo).

## Critérios de aceite

1. Integração: jogo só de mundos aparece com sessões, duração média, abandono e jogadores únicos.
2. Transport e BFF serializam/repassam os campos novos (testes).
3. `tsc` e build do admin-ui verdes.

## Fora de escopo

Backfill de `game_player_seen` para participações antigas (os dados antigos seguem sem jogador visto; o total se
corrige com o uso).
