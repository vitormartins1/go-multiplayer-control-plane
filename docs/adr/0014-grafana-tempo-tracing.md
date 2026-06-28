# ADR-0014: Grafana Tempo + OTLP para distributed tracing

**Status:** Accepted
**Data:** 2026-06-28
**Serviço(s) afetado(s):** todos, infra
**Detalha:** ADR-0006 (que deixou o pilar de traces para sessão própria)

---

## Contexto

Logs e métricas já estão de pé (ADR-0006): `slog` JSON, métricas RED no `/metrics`, dashboards Grafana. Falta o terceiro pilar — **traces**. Hoje só há um serviço (identity), então um trace cobre um hop; mas o valor real aparece quando o fluxo cruza serviços (client → identity → session → game server) e quando se quer responder *"onde foram os 200ms desse request?"* sem ler logs de N serviços.

Duas restrições já decididas em `OBSERVABILITY.md` que esta ADR formaliza:

1. **Propagação W3C trace context** (`traceparent`) ponta a ponta — incluindo carregar o context no connect token / handshake de sessão para o trace continuar até o Unity Dedicated Server (OTel é poliglota: Go + C#).
2. O backend de traces deve caber no **painel único** do Grafana, alimentando a tese custo/performance (self-hosted vs. AWS gerenciado).

A instrumentação precisa nascer padronizada em `shared/telemetry`, como o `shared/middleware` fez com métricas — para qualquer serviço novo herdar tracing sem refazer.

## Alternativas Consideradas

### Opção A: Grafana Tempo (via OTLP)
- **Prós:** integra ao Grafana já existente (datasource nativo) → painel único, sem outra UI; **correlação trace↔métrica↔log** (exemplars no gráfico de latência levam ao trace; trace leva ao log via `trace_id`); barato de operar (storage em objeto/disco, sem índice pesado); self-hosted, alinhado à tese; ingere OTLP padrão.
- **Contras:** UI de busca de traces menos rica que o Jaeger; mais um componente no compose.

### Opção B: Jaeger
- **Prós:** UI de busca/análise de traces madura e dedicada; padrão consolidado.
- **Contras:** fica **fora do Grafana** — quebra o painel único e a correlação direta com métricas/logs; sobreposição de UIs.

### Opção C: AWS X-Ray
- **Prós:** zero infra para manter na AWS; integrado ao ecossistema.
- **Contras:** vendor lock-in; contraria a tese de medição neutra (mesmo motivo que descartou CloudWatch no ADR-0006); pobre/ausente no ambiente local.

## Decisão

Adotamos **Grafana Tempo** como backend de traces, alimentado via **OTLP** (gRPC) pelo **OpenTelemetry Go SDK**. Descartamos o Jaeger por ficar fora do Grafana (perde correlação e painel único) e o X-Ray por lock-in e por contrariar a medição neutra da tese.

Decisões de instrumentação que acompanham:

- **Propagação:** W3C Trace Context (`traceparent`) — propagator global do OTel.
- **Setup padronizado:** `shared/telemetry` expõe um `TracerProvider` configurável (service name, endpoint OTLP, sampler) que todo serviço inicializa no `main.go` e encerra com `Shutdown(ctx)` no graceful shutdown.
- **Instrumentação HTTP:** `otelhttp` no transport (server) + injeção do context nos clients HTTP/gRPC de saída.
- **Sampling:** `ParentBased` — local `AlwaysSample` (ver tudo); prod via env (`TraceRatioBased`, ex. 0.1) para custo previsível.
- **Exemplars:** ligar exemplars no client Prometheus para o gráfico de latência saltar direto ao trace.

## Consequências

### Positivas
- Diagnóstico de latência por span sem cruzar logs de N serviços; painel único Grafana (métrica → trace → log).
- `shared/telemetry` faz todo serviço novo nascer com tracing, como já acontece com métricas.
- Stack se estende ao game server (OTel C#) e ao Agones sem retrabalho.

### Negativas / Trade-offs
- Mais um componente (Tempo) para subir/manter no compose.
- Busca de traces no Grafana é mais pobre que a UI do Jaeger.
- Custo de instrumentar saídas (propagar context em cada client) recai sobre cada serviço.

### Neutras / Implicações
- Abre uma dívida de design no **session-service / connect token**: carregar `traceparent` no handshake para o trace seguir até o Unity DS (entra na spec daquele serviço).
- `TraceRatioBased` em prod implica padronizar a env de sampling em `shared/config`.

## Referências
- Grafana Tempo — grafana.com/docs/tempo
- OpenTelemetry Go — opentelemetry.io/docs/languages/go
- W3C Trace Context — w3.org/TR/trace-context
- Exemplars (Prometheus/Grafana) — grafana.com/docs/grafana/latest/fundamentals/exemplars
