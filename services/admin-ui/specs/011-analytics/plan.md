# Plan: 011-analytics

**Spec:** specs/011-analytics/spec.md
**Data:** 2026-07-04

## Abordagem técnica

Extrai o `StatCard` (hoje privado da Overview, spec 010) para `components/StatCard.tsx` e o reusa nos totais da Analytics. A página consome `GET /api/analytics/overview` via `useAsync` e renderiza totais (cards) + tabela de jogos ordenada.

## Arquivos

- `src/components/StatCard.tsx` (novo) — extrai `{ label, value, bad? }` da Overview; a Overview passa a importar de lá.
- `src/lib/api.ts` — `AnalyticsGameStat`, `AnalyticsOverview`; `api.analyticsOverview()`.
- `src/pages/Analytics.tsx` — remove `ComingSoon`; `useAsync(api.analyticsOverview)`; totais em `StatCard` (partidas, jogadores únicos, jogos) + `Table` de jogos (nome, partidas, únicos, vitórias, score médio); estados loading/erro/vazio.

## Decisões

- **StatCard compartilhado** (não duplicar) — Overview e Analytics usam o mesmo.
- **Sem polling** na Analytics (dado agregado muda devagar; recarrega ao entrar). Diferente dos sinais "ao vivo" do Infrastructure.
- Score médio formatado com 1 casa; nome do jogo do BFF (gameName), fallback pro gameId.

## Ordem
1. `StatCard.tsx` + ajusta Overview. 2. `api.ts`. 3. `Analytics.tsx`. 4. `npm run build` + verificação visual.
