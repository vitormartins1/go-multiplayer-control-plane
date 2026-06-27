# ADR-0006: Prometheus + Grafana + OpenTelemetry como stack de observabilidade

**Status:** Accepted
**Data:** 2026-06-27
**Serviço(s) afetado(s):** todos, infra

---

## Contexto

O projeto precisa de logs, métricas e traces — tanto para operar/depurar localmente quanto para **medir** o trade-off central da tese (Go self-hosted vs. serviços AWS gerenciados). A observabilidade aqui não é enfeite: é o instrumento da comparação de custo/performance/eficiência.

## Alternativas Consideradas

### Opção A: Prometheus + Grafana + OpenTelemetry
- **Prós:** portável, sem vendor lock-in; roda local (Docker) e em qualquer compute; rico para dashboards de comparação; padrão de mercado.
- **Contras:** stack auto-gerenciada (mais peças para subir e manter).

### Opção B: CloudWatch nativo
- **Prós:** zero setup na AWS; integrado.
- **Contras:** acopla à AWS; contraria a tese de medir de forma neutra; custo de logs/métricas; pobre no ambiente local.

## Decisão

Adotamos **Prometheus** (métricas) + **Grafana** (dashboards) + **OpenTelemetry** (traces) + `slog` JSON (logs). A mesma stack roda local (Docker Compose) e em prod, e alimenta os dashboards de comparação (custo, latência p50/p95/p99, throughput, uso de recurso) frente às estimativas dos equivalentes AWS.

O detalhamento da observabilidade local (dashboards iniciais, logs por serviço, visualização de chamadas em testes locais/rede) será tratado numa sessão própria — não altera esta decisão.

## Consequências

### Positivas
- Medição neutra e portável; instrumento direto da tese; aprendizado de stack padrão de mercado.

### Negativas / Trade-offs
- Mais componentes para subir/manter que o CloudWatch nativo.

### Neutras / Implicações
- Setup de telemetry vive em `shared/telemetry` para padronizar instrumentação entre serviços.
- **Cada serviço nasce instrumentado desde o serviço 1** (observabilidade incremental, não uma fase no fim). O stack local (Prometheus + Grafana) sobe junto do primeiro serviço.
- Dashboards Grafana específicos de Go (goroutines, GC, heap, latência p50/p95/p99) serão pesquisados/adaptados ao montar o serviço 1. Go expõe métricas de runtime quase de graça via o client Prometheus (`go_goroutines`, `go_gc_duration_seconds`, `go_memstats_*`) + pprof para profiling.

## Referências
- Prometheus Go Client — prometheus.io/docs/guides/go-application
- OpenTelemetry Go — opentelemetry.io/docs/languages/go
