# Tasks: 009-analytics

**Plan:** specs/009-analytics/plan.md
**Data:** 2026-07-04

- [x] `internal/domain/analytics.go` — `AnalyticsOverview`/`AnalyticsGameStat`/`AnalyticsTotals`; Port `AnalyticsReader`; `ErrAnalyticsUnavailable`
- [x] `internal/analytics/client.go` — `GET /admin/analytics/overview` (+ httptest)
- [x] `internal/service/admin_service.go` — `Deps.Analytics` + `AnalyticsOverview` (enrich nome, best-effort) (+ teste)
- [x] `internal/transport/http_handler.go` — `AdminService` +1; rota `GET /api/analytics/overview` (+ teste)
- [x] `cmd/server/main.go` — `ANALYTICS_URL` + client no `Deps`; compose `admin-bff` ganha `ANALYTICS_URL`
- [x] `go test ./services/admin-bff/... -race` + `go build ./...`
