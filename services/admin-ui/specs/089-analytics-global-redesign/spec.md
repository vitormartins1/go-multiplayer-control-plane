# Spec: 089-analytics-global-redesign

**Serviço:** admin-ui
**Status:** Implemented
**Data:** 2026-07-14
**Nota de processo:** escrita depois da implementação — mesmo caso das
specs 071 a 088. Único item desta rodada de review com plano apresentado
por escrito e aprovado pelo Vitor ANTES do código (os demais itens da
mesma review — 088 — já vinham com "pode atuar", implementação direta).

## Contexto

O Analytics global (`Analytics.tsx`, cross-jogo, `/analytics`) ficou pra
trás da evolução que o Analytics por-jogo (`GameAnalytics.tsx`) passou
nas specs 067/081/086: virou o layout padrão de dashboard SaaS (3 KPI
cards → line chart → tabela), sem `ChannelStrip`, sem capa dos jogos, sem
o par "stats+visual / gráfico por dia" lado a lado que o resto do console
já usa. Apontado no design review (`/frontend-design:frontend-design`)
como o "cheiro mais genérico" do console.

## Plano aprovado

1. `ChannelStrip` (`IconActivity`, label "Geral", readout = total de
   partidas) no lugar do `Title` solto.
2. Par lado a lado 8/4 (mesmo split da spec 086): à esquerda, `Paper` com
   os 3 `StatCard` (`size="lg"`) + um `DonutChart` novo — distribuição de
   partidas por jogo (`gameShareData`, paleta cíclica `GAME_COLORS`,
   já que a quantidade de jogos é dinâmica, ao contrário dos donuts do
   Analytics por-jogo que têm categorias fixas tipo vitória/derrota); à
   direita, `Paper` com o `LineChart` "Partidas por dia".
3. Tabela "Jogos por partidas" ganha `GameCover variant="mini"` na 1ª
   coluna (mesmo padrão de `Rooms.tsx`) — precisa de `api.games()` à
   parte pra montar `gamesById` (o `AnalyticsGameStat` só tem `gameId`,
   não a capa).
4. Estado vazio (`data.games.length === 0`) usa `EmptyState` (spec 088)
   em vez de texto solto.

## Critérios de Aceite

1. `tsc --noEmit` limpo.
2. Layout visualmente alinhado ao Analytics por-jogo — validado ao vivo
   (screenshot desktop 1400px): `ChannelStrip`, par 8/4 com donut ao lado
   dos stats, tabela com capas mini.
3. Donut de distribuição por jogo reflete os dados reais (Packet Arena
   26 partidas = maior fatia) — validado ao vivo.
4. Responsivo em mobile (390px): par empilha, `Group` do donut+stats
   também empilha (mesmo `wrap="wrap"` da spec 086) — validado ao vivo.
5. Sem erro de console.

## Fora de escopo

- Mudança de endpoint/dado — `AnalyticsOverview` já tinha tudo necessário
  (cruza com `api.games()` no cliente, mesmo padrão de `Rooms.tsx`).
- Filtro por jogo/modo nesta página — ela é deliberadamente cross-jogo; o
  detalhe por jogo já existe na aba Analytics de cada jogo.
