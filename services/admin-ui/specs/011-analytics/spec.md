# Spec: 011-analytics

**Serviço:** admin-ui
**Status:** Done (2026-07-04)
**Data:** 2026-07-04
**Parte de:** Analytics (pares: `analytics-service/001-match-analytics`, `admin-bff/009-analytics`)

## Contexto

Substitui o `ComingSoon` da aba **Analytics** por um **overview cross-game** real, alimentado pelo analytics-service via BFF (ADR-0021).

## Comportamento

- THE SYSTEM SHALL, na página **Analytics** (rota `/analytics`, já existente), consumir `GET /api/analytics/overview` e mostrar:
  - **Totais** no topo (cards `StatCard`, reuso da 010): partidas, jogadores únicos, jogos com atividade.
  - **Tabela de jogos** ordenada por partidas: Jogo (nome), Partidas, Jogadores únicos, Vitórias, Score médio.
- THE SYSTEM SHALL tratar carregando/erro/vazio explicitamente (sem placeholder piscando), como as telas atuais; analytics fora -> alerta degradado.
- Remove o `ComingSoon` da Analytics.

## Contratos de Interface (cliente)

- `src/lib/api.ts`: `AnalyticsOverview` + `api.analyticsOverview()` -> `GET /api/analytics/overview`.
- `src/pages/Analytics.tsx`: passa a renderizar o overview (reuso de `StatCard`).

## Casos de Teste / Verificação

- [ ] Analytics mostra os totais + tabela ordenada por partidas com nome do jogo
- [ ] estados carregando/erro/vazio corretos; analytics fora -> degrada sem quebrar
- [ ] `npm run build` (typecheck) limpo

## Fora do escopo

- Gráficos temporais (série daily existe no serviço; fica pra iteração/Grafana).
- Deep-dive por jogo (aba no GameDetail) — futuro.
