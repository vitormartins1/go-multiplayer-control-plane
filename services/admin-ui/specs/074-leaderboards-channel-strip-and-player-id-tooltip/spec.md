# Spec: 074-leaderboards-channel-strip-and-player-id-tooltip

**Serviço:** admin-ui
**Status:** Implemented
**Data:** 2026-07-13
**Nota de processo:** escrita depois da implementação — mesmo caso das
specs 071/072/073.

## Contexto

Duas frentes na revisão da aba Players/Leaderboards:

1. O usuário pediu explicitamente pra usar o "channel strip" da Analytics
   (spec 071) como inspiração nos grupos de board da "Visão geral" do
   Leaderboard — o `Title order={5}` da spec 073 (bom, mas mais neutro)
   não era o que ele tinha em mente; ele queria o elemento mais marcante:
   badge do ícone + régua + leitura mono ao final ("tipo 26 partidas").
2. Ao revisar formatação de player ID entre tabelas (parte da revisão
   "Players"): `GameLeaderboards.tsx` (tabela única) era a ÚNICA tabela
   do console mostrando o UUID **inteiro**, sem truncar — `guest-<uuid>`
   tem 43 caracteres e estourava a linha. Achievements/Players/
   GamePlayers já truncavam pra 8 chars; só o drawer de progresso de
   conquista tinha o Tooltip com o ID completo no hover — Players.tsx e
   GamePlayers.tsx truncavam sem essa opção.

## Decisões

- **ChannelStrip extraído como componente compartilhado**
  (`src/components/ChannelStrip.tsx` + `.css`, movido de dentro de
  `GameAnalytics.tsx`) — a 2ª aplicação real (Leaderboards) justifica a
  abstração agora, não antes. `GameAnalytics.tsx` passa a importar o
  componente compartilhado; comportamento idêntico (validado
  visualmente, sem regressão).
- **Leaderboards, modo "Visão geral"**: cada grupo de board usa
  `ChannelStrip` no lugar do `Title order={5}` da spec 073. Ícone único
  (`IconChartBar`, o mesmo já usado na aba Leaderboards) pra todo board —
  sem taxonomia de ícone por tipo de board (diferente do arquétipo em
  Analytics, que já tem `ARCHETYPE_META`); a diferenciação vem do
  rótulo. Leitura = contagem de jogadores no ranking "Geral" (all-time)
  daquele board (`limit=50`, mostra "50+" se bateu o teto) — busca leve
  adicional por grupo, only-`alltime`/`limit=50`, sem paginar.
- **Player ID truncado + Tooltip, padronizado**: `GameLeaderboards.tsx`
  (tabela única) passa a truncar pra 8 chars como as outras; as 3
  tabelas que já truncavam (`Players.tsx`, `GamePlayers.tsx`,
  `GameLeaderboards.tsx`) ganham o `Tooltip` com o ID completo, igual ao
  que já existia no drawer de progresso do Achievements.

## Fora de escopo

- Mini-tabelas da "Visão geral" (`MiniBoardTable`): continuam sem
  channel strip — são o conteúdo DENTRO de cada canal, não uma seção
  própria.
- Modo "Específica" (tabela única paginada): não ganha channel strip —
  já tem seu próprio cabeçalho de página ("Ranking do jogo") e o board
  selecionado já está nos controles do toolbar.

## Critérios de Aceite

1. "Visão geral" (single board ou "Todas"): cada grupo de board mostra
   badge + régua + leitura ("N jogadores"), mesma linguagem visual do
   Analytics.
2. Nenhuma tabela do console mostra um UUID de player não-truncado.
3. Toda tabela com ID truncado tem o ID completo disponível no hover
   (Tooltip).
4. Analytics (`GameAnalytics.tsx`) sem regressão visual após a extração
   do componente compartilhado.
5. Sem erro de console.
