# Spec: 070-session-closed-daily-card

**Serviço:** admin-ui
**Status:** Done (implementada e mergeada; status corrigido na revisão 2026-09)
**Data:** 2026-07-13
**Parte de:** analytics-service/015-session-closed-daily +
admin-bff/028 (passthrough).

## Contexto

`GameAnalyticsDetail` ganha `sessionCloseDaily`. Falta o card no catálogo
(mesmo padrão `DailyLineChart` já reusado 4x).

## Comportamento

- Novo card `session-closed-daily` ("Sessões fechadas por dia"), mesmo
  gating por presença de dado dos outros (`data.sessionCloseDaily.length >
  0`).
- Reorganiza a seção final (cross-arquétipo): `session-health` (span 6) +
  `session-closed-daily` (span 6) pareados lado a lado (mesmo padrão
  "distribuição + diário" das outras seções); `raw-stats` passa pra
  `span 12`, sozinho, por não ter um par natural (cross-arquétipo por
  natureza, sem uma "distribuição" pra combinar).
- Sem ícone de arquétipo (mesmo tratamento de `raw-stats`/`session-health`
  — cross-arquétipo).

## Contratos de Interface

```ts
// lib/api.ts
export interface GameAnalyticsDetail {
  // ... campos existentes
  sessionCloseDaily: DailyPoint[];
}
```

## Critérios de Aceite

1. Jogo com `sessionCloseDaily` -> card aparece pareado com "Saúde das
   sessões"; `raw-stats` full-width acima.
2. Filtro por modo -> segue funcionando (sessionCloseDaily já filtrado
   pelo backend).
3. Sem dado -> card não aparece, layout cai pra 1 card só na seção final
   (mesmo comportamento gating de sempre).

## Fora de escopo

Nenhum.
