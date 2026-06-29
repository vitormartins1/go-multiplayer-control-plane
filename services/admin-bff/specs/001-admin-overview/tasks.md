# Tasks: 001-admin-overview
**Plan:** specs/001-admin-overview/plan.md
**Data:** 2026-06-29

> Novo serviço Go REST, agregador read-only (ADR-0016). Sem banco.
> Ordem: domain → prometheus client → achievements client → service → transport → wire-up/infra.
> Depende da feature achievements/003-admin-read para o painel de achievements.
> Marcar [x] conforme implementa.

## Fase 1 — Domain
- [ ] `internal/domain/admin.go` — tipos `ServiceNode`, `Edge`, `Topology`, `LiveMetrics`, `PlayerAchievement`, `Sample`
- [ ] `internal/domain/admin.go` — Ports `MetricsSource` (`Query(ctx, promQL) ([]Sample, error)`) e `AchievementsReader` (`ForPlayer(ctx, playerID) ([]PlayerAchievement, error)`)
- [ ] `internal/domain/topology.go` — topologia conhecida (nós dos serviços Go + arestas de dependência) como dados estáticos

## Fase 2 — Prometheus client (adapter MetricsSource)
- [ ] `internal/prometheus/client.go` — `GET {PROMETHEUS_URL}/api/v1/query?query=...`; parseia `data.result[].metric` + `value` (valor vem como string); erro de conexão distinguível (fonte indisponível)
- [ ] `internal/prometheus/client_test.go` — `httptest`: parse de `up{job=...}` e de um contador; resposta de erro/timeout -> erro de fonte

## Fase 3 — Achievements client (adapter AchievementsReader)
- [ ] `internal/achievements/client.go` — `GET {ACHIEVEMENTS_URL}/admin/players/{id}/achievements`; mapeia o JSON para `[]PlayerAchievement`
- [ ] `internal/achievements/client_test.go` — `httptest`: mapear resposta com/sem progresso; serviço fora -> erro

## Fase 4 — Service
- [ ] `internal/service/admin_service.go` — `Topology(ctx)` (query `up{}` -> nós + arestas estáticas; degrada se a fonte cair), `LiveMetrics(ctx)` (contadores em paralelo via `errgroup`), `PlayerAchievements(ctx, id)` (delega ao reader)
- [ ] `internal/service/admin_service_test.go` — fakes das Ports (`-race`): topologia com up/down; Prometheus indisponível -> nós `unknown` + `sourceAvailable=false`; live agrega; achievements delega

## Fase 5 — Transport
- [ ] `internal/transport/http_handler.go` — rotas `/api/topology`, `/api/metrics/live`, `/api/players/{playerID}/achievements`; mapear erro de fonte -> 200(degrada)/503/502
- [ ] `internal/transport/http_handler_test.go` — `httptest`: topologia ok + degradada (200); live ok + fonte fora (503); achievements ok + fonte fora (502)

## Fase 6 — Wire-up, infra e verificação
- [ ] `cmd/server/main.go` — config (`PROMETHEUS_URL`, `ACHIEVEMENTS_URL`); clients HTTP com `otelhttp`; `middleware.NewMetrics` + `RouteSpanName`; `/healthz`, `/metrics`; graceful shutdown
- [ ] `services/admin-bff/Dockerfile` + `api/README.md` (curl) + `README.md`
- [ ] `infra/docker/docker-compose.yml` — serviço `admin-bff` (`PROMETHEUS_URL`, `ACHIEVEMENTS_URL`, `depends_on`) + `infra/docker/prometheus.yml` — job `admin-bff`
- [ ] `go test ./... -race` verde; `go build ./...`
- [ ] Validar local: `/api/topology` (nós com up), `/api/metrics/live` (contadores), `/api/players/{id}/achievements` (catálogo anotado); Prometheus fora -> topologia degrada
