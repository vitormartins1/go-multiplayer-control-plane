# Spec: 024-analytics-fase2-passthrough

**Serviço:** admin-bff
**Status:** Done (2026-07-13) — validado ao vivo via curl no admin-bff
**Data:** 2026-07-13
**Parte de:** analytics-service/010-mode-scoped-time-series + 011-raw-stat-capture ("Analytics Fase 2/3").

## Contexto

O analytics-service ganha 3 séries diárias novas (`placementDaily`/
`missionDaily`/`participationDaily`) e um resumo agregado de stats brutos
(`stats[]`) no mesmo endpoint já existente (`GET /admin/games/{gameId}/
analytics`). O admin-bff tem um tipo espelho (`domain.GameAnalyticsDetail`),
não passthrough cru — precisa dos campos novos pra não descartá-los na
decodificação.

## Comportamento

- `GameAnalyticsDetail` (admin-bff) SHALL ganhar os mesmos 4 campos aditivos
  do analytics-service: `placementDaily`, `missionDaily`, `participationDaily`
  (cada um `[]AnalyticsDaily`, reaproveitando o tipo já existente) e `stats`
  (`[]StatSummary`, tipo novo espelho de `analytics-service.StatSummary`).
- Nenhuma rota nova — o mesmo `GET /api/analytics/games/{gameId}` (admin-bff)
  passa a devolver os campos novos (o `Client.GameAnalytics` já decodifica por
  nome de campo JSON; só precisa dos campos existirem no struct Go).
- Sem enriquecimento adicional do BFF nesses campos (não há handle nem nome
  de jogo a resolver aqui — são só números agregados).

## Casos de Teste / Verificação

- [x] `GameAnalyticsDetail` decodifica `placementDaily`/`missionDaily`/
      `participationDaily`/`stats` do JSON do analytics-service sem perder
      dado (teste do client com fixture JSON incluindo os campos novos)
- [x] Resposta de um jogo sem esses dados (campos ausentes/vazios no upstream)
      não quebra a decodificação — campos ficam com slice vazio
