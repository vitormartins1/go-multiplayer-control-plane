# Tasks: 002-service-metrics
**Plan:** specs/002-service-metrics/plan.md
**Data:** 2026-06-29

> Extensão do admin-bff/001: reusa MetricsSource + KnownServices + errgroup. Sem novo adapter.
> Marcar [x] conforme implementa.

## Fase 1 — Domain
- [ ] `internal/domain/admin.go` — tipo `ServiceMetrics{ ID string; Up, RequestRate, ErrorRate, LatencyP95, Goroutines, MemoryBytes *float64 }`
- [ ] `internal/domain/errors.go` — `ErrUnknownService`
- [ ] `internal/domain/topology.go` — helper `IsKnownService(id string) bool`

## Fase 2 — Service
- [ ] `internal/service/admin_service.go` — `ServiceMetrics(ctx, id)`: valida `id` (senão `ErrUnknownService`); roda up + RED (rate/erro/p95) + runtime (goroutines/memória) em `errgroup`; reusa `scalar`
- [ ] `internal/service/admin_service_test.go` — REST conhecido (up+RED+runtime); gRPC (RED nil); métrica ausente (nil); id desconhecido (`ErrUnknownService`); Prometheus fora (`ErrMetricsUnavailable`); `-race`

## Fase 3 — Transport
- [ ] `internal/transport/http_handler.go` — rota `GET /api/services/{id}/metrics`; mapear `ErrUnknownService`->404, `ErrMetricsUnavailable`->503
- [ ] `internal/transport/http_handler_test.go` — 200 (REST), 200 com RED null (gRPC), 404 (id desconhecido), 503 (fonte fora)

## Fase 4 — Docs e verificação
- [ ] `api/README.md` + `README.md` — documentar `GET /api/services/{id}/metrics`
- [ ] `go test ./... -race` verde; `go build ./...`
- [ ] Validar local (Prometheus fake): REST com RED; gRPC sem RED; id inválido -> 404; fonte fora -> 503
