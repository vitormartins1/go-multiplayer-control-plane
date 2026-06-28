# Observabilidade local

> Stack local de observabilidade do backend Go + banco. Cada serviço **nasce instrumentado** (ADR-0006); a stack é também o instrumento de medição da tese de comparação (Go self-hosted vs. AWS gerenciado).

## O que está incluído

| Pilar | Como |
|---|---|
| **Logs** | `slog` JSON no stdout de cada serviço |
| **Métricas** | Prometheus client no serviço → `GET /metrics` → Prometheus → Grafana |
| **Métricas do banco** | `postgres_exporter` → Prometheus |
| **Traces** | OpenTelemetry (a integrar quando houver fluxo entre serviços) |

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
| Prometheus | http://localhost:9090 | ver alvos em Status → Targets |
| Grafana | http://localhost:3000 | login anônimo (admin); datasource Prometheus já provisionado |
| postgres_exporter | http://localhost:9187/metrics | métricas do Postgres |

Gerar tráfego para ver as métricas RED:

```bash
curl -s localhost:8081/healthz
curl -s -XPOST localhost:8081/accounts -d '{"handle":"Vitor_1","password":"supersecret"}'
```

## Dashboards recomendados (importar via Grafana UI → Import by ID)

A datasource Prometheus já está provisionada; ao importar, selecione-a.

| Dashboard | ID | Para |
|---|---|---|
| Go Processes / Go Metrics | 6671 | goroutines, GC, heap, threads |
| PostgreSQL (postgres_exporter) | 9628 | conexões, throughput, locks do banco |

Para o RED do próprio serviço, montar painéis com:
- RPS: `sum(rate(http_requests_total[1m])) by (route)`
- Erro: `sum(rate(http_requests_total{status=~"5.."}[5m]))`
- Latência p95: `histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le, route))`

> Dashboards provisionados (JSON versionado) podem ser adicionados depois, validados em Grafana. Por ora, importação via UI mantém o setup simples.
