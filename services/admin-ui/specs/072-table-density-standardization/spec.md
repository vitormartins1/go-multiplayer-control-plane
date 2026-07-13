# Spec: 072-table-density-standardization

**Serviço:** admin-ui
**Status:** Implemented
**Data:** 2026-07-13
**Nota de processo:** escrita depois da implementação — mesmo caso da
spec 071 (sessão de exploração/ajuste visual, não passou pelo gate normal
de "spec antes de implementar").

## Contexto

Pedido do usuário: usar a tabela de "Catálogo de conquistas"
(`GameAchievements.tsx`) como referência de tamanho/densidade e igualar
as outras tabelas das abas do jogo que "ficam um pouco menores" —
Leaderboards e Sessions nomeadas explicitamente.

Medição real (Playwright, computed styles) mostrou que **font-size (14px)
e padding de célula (7px 10px) já eram idênticos em todas** — a
diferença de densidade percebida vinha inteiramente do tamanho do
ícone/avatar de cada linha, que arrasta a altura da linha junto:

| Tabela | Ícone antes | Row height antes |
|---|---|---|
| Achievements (referência) | 32px | 47px |
| Players (aba do jogo) | 32px | 47px (já igual) |
| Players (global, `/players`) | 32px | 47px (já igual) |
| Leaderboards (tabela única) | 24px | 43.7px |
| Sessions | 24px | 39px |

## Comportamento

- `GameLeaderboards.tsx`, tabela única ("Específica"): `PlayerAvatar`
  24px → 32px.
- `GameSessions.tsx`: `SessionAvatar` da coluna Match 24px → 32px;
  `PlayersSummary` (avatares empilhados da coluna Players) 24px → 32px.
- `PlayersSummary` ganhou prop `size` (default 24, preserva quem não
  passa) — o overlap dos avatares empilhados escala com o tamanho
  (`-round(size/3)`).
- Resultado: as 4 tabelas (Achievements/Players-aba/Leaderboards/
  Sessions) têm agora `rowHeight: 47px` e ícone `32px` idênticos
  (validado via computed styles).

## Fora de escopo (deliberado, não mexido)

- `MiniBoardTable` (Leaderboards, modo "Visão geral"): mantém 20px —
  grid de várias mini-tabelas lado a lado, contexto de densidade
  diferente (comparar 4-8 mini-boards ao mesmo tempo).
- `Rooms.tsx` (view cross-jogo, nav "Rooms"): mantém 16px — tabela
  operacional de alto volume (45+ linhas, todos os jogos ao mesmo
  tempo, mais colunas); usuário não citou essa página, e igualar a
  densidade da referência atropelaria a escaneabilidade num contexto de
  volume bem maior.
- `SessionMatchDetail.tsx` (painel expandido de uma sessão): avatar
  já usa 40px, maior que a referência — não é uma linha de tabela, é
  um header de detalhe, contexto diferente.

## Critérios de Aceite

1. Achievements, Players (aba do jogo), Leaderboards (tabela única) e
   Sessions têm a mesma altura de linha e o mesmo tamanho de ícone.
2. Visão geral do Leaderboard (mini-tabelas) e Rooms permanecem
   inalterados.
3. Sem erro de console, sem regressão de layout (validado com Playwright
   em `e4c9c5b4` / Packet Arena, dado real).
