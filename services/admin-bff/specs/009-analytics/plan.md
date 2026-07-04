# Plan: 009-analytics

**Spec:** specs/009-analytics/spec.md
**Data:** 2026-07-04

## Abordagem técnica

Adapter fino p/ o analytics-service + enrich de nome do jogo no service (reuso do `gameNamesByID` da 008). Mesmo shape dos outros adapters (URL + httpClient com internal key).

## Arquivos

- `internal/domain/analytics.go` (novo) — `AnalyticsGameStat`, `AnalyticsTotals`, `AnalyticsOverview`; Port `AnalyticsReader.Overview(ctx)`; `ErrAnalyticsUnavailable`.
- `internal/analytics/client.go` (novo) — `GET /admin/analytics/overview`; indisponibilidade → `ErrAnalyticsUnavailable`.
- `internal/service/admin_service.go` — campo `analytics` + `Deps.Analytics`; `AnalyticsOverview(ctx)` = lê + injeta `gameName` (best-effort via `gameNamesByID`).
- `internal/transport/http_handler.go` — `AdminService` +`AnalyticsOverview`; rota `GET /api/analytics/overview`; `writeUpstream`/`ErrAnalyticsUnavailable`→502.
- `cmd/server/main.go` — `ANALYTICS_URL` + `analyticsclient.New` no `Deps`.
- Testes: adapter (httptest), service (enrich), transport.

## Decisões

- **Enrich best-effort**: registry fora → overview sem nome, não falha (igual Rooms).
- **502** quando analytics fora (`ErrAnalyticsUnavailable`), como os outros upstreams.
- Read por-jogo não exposto no BFF v1 (painel só usa o overview).

## Ordem
1. domain. 2. adapter (+httptest). 3. service (+teste). 4. transport (+teste) + main. 5. `go test -race` + build.
