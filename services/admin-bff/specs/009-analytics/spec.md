# Spec: 009-analytics

**Serviço:** admin-bff
**Status:** Done
**Data:** 2026-07-04
**Parte de:** Analytics (pares: `analytics-service/001-match-analytics`, `admin-ui/011-analytics`)

## Contexto

Agrega para o painel o **overview cross-game** do analytics-service (ADR-0021), enriquecendo os jogos com o **nome** (game-registry) — o analytics só conhece `gameId`, como o session no Rooms (spec 008).

## Comportamento

- WHEN `GET /api/analytics/overview` -> THE SYSTEM SHALL ler `GET /admin/analytics/overview` do analytics (internal key), enriquecer cada jogo com `gameName` (via `gameNamesByID`, reuso da 008) e devolver `{ totals, games[] }`. Analytics fora -> `ErrAnalyticsUnavailable` (502). Enrich best-effort (registry fora -> sem nome, não falha).

## Contratos de Interface

- **Domain:** `AnalyticsOverview` (`totals` + `games[]{gameId, gameName, matches, uniquePlayers, wins, avgScore}`); Port `AnalyticsReader.Overview(ctx)`; `ErrAnalyticsUnavailable`.
- **Adapter `analytics/client.go`** (novo) — satisfaz `AnalyticsReader`; `GET /admin/analytics/overview`.
- **Service:** `AnalyticsOverview(ctx)` — lê + `gameNamesByID` (best-effort).
- **Transport:** `GET /api/analytics/overview`.

## Casos de Teste Obrigatórios

- [ ] agrega o overview e injeta `gameName` por jogo
- [ ] analytics fora -> 502; registry fora -> overview sem nome (best-effort)

## Fora do escopo

- Read por-jogo (`/admin/games/{id}/analytics` existe no serviço; o painel v1 só usa o overview).
