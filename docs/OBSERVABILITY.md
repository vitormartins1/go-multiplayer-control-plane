# Observabilidade local

> Stack local de observabilidade do backend Go + banco. Cada serviço **nasce instrumentado** (ADR-0006); a stack é também o instrumento de medição da tese de comparação (Go self-hosted vs. AWS gerenciado).

## O que está incluído

| Pilar | Como |
|---|---|
| **Logs** | `slog` JSON no stdout de cada serviço |
| **Métricas** | Prometheus client no serviço → `GET /metrics` → Prometheus → Grafana |
| **Métricas do banco** | `postgres_exporter` → Prometheus |
| **Traces** | OpenTelemetry (W3C trace context) → OTLP gRPC → **Grafana Tempo** → Grafana (ADR-0014) |

Métricas expostas pelo serviço Go em `/metrics`:

- **RED (nossas):** `http_requests_total{method,route,status,service}`, `http_request_duration_seconds{method,route,service}` — o label `route` usa o pattern do ServeMux (baixa cardinalidade).
- **Runtime do Go (de graça):** `go_goroutines`, `go_gc_duration_seconds`, `go_memstats_*`, `process_*`.

## Como rodar

```bash
docker compose -f infra/docker/docker-compose.yml up --build
```

| Serviço | URL | Notas |
|---|---|---|
| identity-service | http://localhost:8081 | API; `/metrics`, `/healthz` |
| achievements-service | http://localhost:8082 | API; `/metrics`, `/healthz` |
| Prometheus | http://localhost:9090 | ver alvos em Status → Targets |
| Grafana | http://localhost:3000 | login anônimo (admin); datasources Prometheus e Tempo provisionados |
| Tempo | http://localhost:3200 | backend de traces (API de query); OTLP gRPC em `:4317` |
| identity-postgres-exporter | http://localhost:9187/metrics | métricas do banco do identity |
| achievements-postgres-exporter | http://localhost:9188/metrics | métricas do banco do achievements |

Gerar tráfego para ver as métricas RED:

```bash
curl -s localhost:8081/healthz
curl -s -XPOST localhost:8081/accounts -d '{"handle":"Vitor_1","password":"supersecret"}'
```

### Ver traces

O `POST /accounts` acima já gera um trace (`/healthz` e `/metrics` ficam de fora, para não poluir). No Grafana → **Explore** → datasource **Tempo** → busca TraceQL `{ resource.service.name = "identity-service" }`. O serviço só emite traces quando `OTEL_EXPORTER_OTLP_ENDPOINT` está setado (o compose aponta para `tempo:4317`); rodando solto via `go run` sem essa env, o tracing fica no-op.

## Dashboards

**Provisionados** (sobem automático na pasta `game-backend`):
- **Service Overview (RED)** — parametrizado por `$job` (um serviço, ou "All" para a frota): RPS / erros 5xx / latência p95 por rota + goroutines / heap / GC. Como o `shared/middleware` padroniza as métricas, este dashboard serve **qualquer serviço Go** sem refazer.
- **PostgreSQL** — parametrizado por `$job` (um banco, ex. `identity-postgres`/`achievements-postgres`, ou "All"): conexões, commits/rollbacks, cache hit, tuplas/s, tamanho do banco, deadlocks. Serve **qualquer `<svc>-postgres-exporter`** sem refazer.
- **Tracing (APM)** — **service graph** (mapa de dependências `user → serviço → banco`, derivado dos traces) + RED por serviço e latência p95 a partir dos spans. Alimentado pelo **metrics-generator do Tempo** (gera `traces_*` e faz `remote_write` ao Prometheus). É o "APM visual"; para inspecionar um trace específico em waterfall, use **Explore → Tempo** (ou a aba **Service Graph** lá).

**Complementar** (importar via Grafana UI → Import by ID; selecionar a datasource Prometheus):

| Dashboard | ID | Para |
|---|---|---|
| Go Processes | 6671 | deep-dive de runtime |

## Padrão por serviço novo

Cada serviço Go nasce instrumentado (ADR-0006). Ao adicionar um serviço, siga a nomenclatura e o checklist — assim ele entra na stack de observabilidade sem refazer dashboard.

### Nomenclatura

| Recurso | Padrão | identity / achievements |
|---|---|---|
| Serviço HTTP (compose) | `<svc>-service` | identity-service / achievements-service |
| Banco (compose) | `<svc>-postgres` | identity-postgres / achievements-postgres |
| Database | `<svc>` | identity / achievements |
| Exporter do banco | `<svc>-postgres-exporter` | identity-postgres-exporter / achievements-postgres-exporter |
| Porta do banco (host) | 5432, 5433, … | 5432 / 5433 |
| Porta do exporter (host) | 9187, 9188, … | 9187 / 9188 |
| `service.name` (OTel) / `job` (Prometheus) | `<svc>-service` | identity-service / achievements-service |

### Checklist de observabilidade (a cada serviço novo)

- [ ] **Métricas RED + runtime:** herdadas do `shared/middleware`; exponha `/metrics` (OpenMetrics) no wire-up. Aparece automático no dashboard **Service Overview (RED)** (parametrizado por `$job`).
- [ ] **Scrape do serviço:** job `<svc>-service` no `prometheus.yml`.
- [ ] **Tracing:** `telemetry.SetupTracing` + `otelhttp` + otelpgx no wire-up (ADR-0014); `OTEL_EXPORTER_OTLP_ENDPOINT` no compose.
- [ ] **Banco:** `<svc>-postgres-exporter` no compose + job `<svc>-postgres` no `prometheus.yml` (dashboard PostgreSQL 9628, filtrar por `instance`).
- [ ] **Logs:** `slog` JSON via `shared/logger` (de graça).

## Visão de futuro: observabilidade do sistema completo

A observabilidade não para no backend Go. Não implementamos agora, mas as decisões atuais já acomodam:

### Game server (Unity Dedicated Server)
- **Métricas:** tick rate, jogadores na partida, duração da partida, CPU/mem — via **OTel C# SDK** → Prometheus (OTel é poliglota: serve Go e C#).
- **Traces:** o trace deve **continuar do backend para o game server**. O handshake de sessão/connect token precisa **carregar o trace context (W3C `traceparent`)**, para um único trace cobrir client → backend → game server. (Decisão de design do session-service + connect token — entra na ADR de tracing.)

### Fleet/cluster de game servers (Agones no k8s)
- **Agones expõe métricas Prometheus** nativas (GameServers por estado, réplicas do Fleet, latência de allocation) + **kube-state-metrics** e métricas de nó do cluster (homelab Pi 5).
- **Dashboards:** Fleet Overview (Ready/Allocated), latência de alocação, saúde do cluster.

### Game clients (se aplicável)
- Telemetria client-side (FPS, RTT ao server, erros) — em boa parte território do **analytics-service**; pode emitir OTel do cliente. Prioridade menor.

### O que isso decide AGORA
- **Tracing:** propagação **W3C trace context** ponta a ponta; desenhar o connect token / handshake de sessão para carregar `traceparent`.
- **Convenções de label:** manter `job`/`service` consistentes (e `match_id` onde fizer sentido) para métricas do game server caberem no mesmo modelo de dashboard.
- **Stack já acomoda:** OTel (Go + C#), Prometheus (nativo no Agones), Grafana (painel único) — as escolhas atuais se estendem ao lado do jogo sem retrabalho.
