# Spec: 002-overview-daily

**Serviço:** analytics-service (+ admin-bff passthrough)
**Status:** Done
**Data:** 2026-07-04
**ADR:** 0021 · **Par:** `admin-ui/013-analytics-chart`

## Contexto

O overview cross-game (spec 001) tem totais + jogos, mas **sem série temporal**. O painel quer um gráfico "partidas por dia". A tabela `game_daily` já guarda matches/dia por jogo — falta **agregar entre jogos** e expor no overview.

## Comportamento

- WHEN `GET /admin/analytics/overview` -> THE SYSTEM SHALL incluir `daily`: `[{day, matches}]` = **soma das partidas por dia entre todos os jogos**, últimos N dias (default 30), ordenado por dia.
- Sem dados -> `daily: []`.

## Contratos de Interface

- **Domain:** `Overview` ganha `Daily []DailyPoint`.
- **Repository:** `Overview` roda `SELECT day, SUM(matches) FROM game_daily WHERE day >= today-N GROUP BY day ORDER BY day`.
- **Transport:** `overviewDTO` ganha `daily`.
- **admin-bff:** `AnalyticsOverview` (domain) ganha `Daily`; o adapter já desserializa o overview inteiro (só o campo novo).

## Casos de Teste Obrigatórios

- [ ] overview inclui `daily` somando partidas/dia de jogos distintos no mesmo dia
- [ ] sem dados -> `daily` vazio

## Fora do escopo
- Janela configurável pelo cliente (fixa em 30 no v1); granularidade < dia.
