# Spec: 002-service-metrics

**Serviço:** admin-bff
**Status:** Approved
**Data:** 2026-06-29

## Contexto

O painel admin (admin-ui/001) passou a ser **organizado por domínio**: cada serviço Go tem sua página com **saúde + RED** (Rate, Errors, Duration). Esta feature adiciona ao `admin-bff` o endpoint que alimenta essas páginas: `GET /api/services/{id}/metrics`, agregando do Prometheus as métricas de um serviço específico. Mesma fonte e padrão do `admin-bff/001` (HTTP API do Prometheus); **sem** tocar nos serviços e **sem** banco.

Nuance honesta de cobertura: as métricas **RED HTTP** vêm de `http_requests_total` / `http_request_duration_seconds`, que **só os serviços REST** expõem (via `shared/middleware`). Os serviços **gRPC** (matchmaking, session) **não** têm RED HTTP — para eles o endpoint devolve `up` + **runtime** (goroutines, memória), com os campos RED `null`. Os contadores de produto (fila, sessions, matches) seguem no `/api/metrics/live` (admin-bff/001) e não se repetem aqui.

## Comportamento

### Métricas de um serviço
- WHEN `GET /api/services/{id}/metrics` com `id` de um serviço conhecido -> consulta o Prometheus e retorna `up`, **RED** (taxa de requisições, taxa de erro, latência p95) e **runtime** (goroutines, memória residente) daquele serviço.
- THE SYSTEM SHALL devolver cada campo como `null` quando a métrica não existir para o serviço (ex.: RED é `null` para os serviços gRPC, que não expõem `http_requests_total`).
- IF `id` não for um serviço conhecido (`domain.KnownServices`) -> `404` (**not_found**).
- IF o Prometheus estiver indisponível -> `503` (**metrics_unavailable**), como no `/api/metrics/live`.
- THE SYSTEM SHALL executar as queries do serviço em paralelo (errgroup), como o `LiveMetrics` do 001.

### Queries (PromQL) usadas
- `up`: `up{job="<id>"}`
- request rate (req/s): `sum(rate(http_requests_total{service="<id>"}[5m]))`
- error rate (fração 0..1): `sum(rate(http_requests_total{service="<id>",status=~"5.."}[5m])) / sum(rate(http_requests_total{service="<id>"}[5m]))`
- latency p95 (s): `histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket{service="<id>"}[5m])) by (le))`
- goroutines: `go_goroutines{job="<id>"}`
- memória (bytes): `process_resident_memory_bytes{job="<id>"}`

> O label `service` (RED) e o label `job` (up/runtime) coincidem com o id do serviço no projeto (ex.: `identity-service`).

## Contratos de Interface

### API HTTP (nova rota)
| Método | Rota | Retorno |
|---|---|---|
| GET | `/api/services/{id}/metrics` | `{ id, up, requestRate, errorRate, latencyP95, goroutines, memoryBytes }` (campos ausentes = `null`) |

### Reuso (sem alteração de Port)
- `domain.MetricsSource.Query(ctx, promQL)` — já existe (admin-bff/001); a feature só compõe novas queries.
- `domain.KnownServices` — já existe; usado para validar o `id`.

### Tipos (domínio)
- `ServiceMetrics{ ID string; Up, RequestRate, ErrorRate, LatencyP95, Goroutines, MemoryBytes *float64 }`

## Casos de Teste Obrigatórios

- [ ] Serviço REST conhecido -> `up` + RED (rate/erro/p95) + runtime preenchidos (Prometheus fake)
- [ ] Serviço gRPC (sem `http_requests_total`) -> RED `null`, `up` + runtime presentes
- [ ] Métrica ausente (sem amostra) -> campo `null` (não zero)
- [ ] `id` desconhecido -> `404`
- [ ] Prometheus indisponível -> `503`
- [ ] As queries rodam em paralelo (errgroup) e o erro de fonte propaga

## Fora do escopo

- **Contadores de produto** (fila/sessions/matches) — já no `/api/metrics/live` (admin-bff/001)
- **Séries temporais / histórico** — só valor instantâneo (gráficos de série ficam no Grafana, ADR-0016)
- **Métricas de banco** (postgres-exporter) por serviço — incremental depois
- **Auth** — interno/local no MVP
