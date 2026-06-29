# Plan: 002-service-metrics
**Spec:** specs/002-service-metrics/spec.md
**Data:** 2026-06-29

## Abordagem técnica

Extensão do `admin-bff/001` (ADR-0002/0016): reusa a Port `domain.MetricsSource` (Prometheus) e `domain.KnownServices`. Adiciona um tipo `ServiceMetrics`, um método de service `ServiceMetrics(ctx, id)` que compõe as queries (up + RED + runtime) em **paralelo via errgroup** (como o `LiveMetrics`), e uma rota `GET /api/services/{id}/metrics` no transport. Sem novo adapter, sem tocar nos serviços, sem banco.

A degradação segue o padrão: erro de fonte (`ErrMetricsUnavailable`) propaga e o transport mapeia para `503`; `id` fora de `KnownServices` -> `404`; métrica sem amostra -> campo `null`.

## Arquivos a criar ou modificar

- `internal/domain/admin.go` — novo tipo `ServiceMetrics{ ID string; Up, RequestRate, ErrorRate, LatencyP95, Goroutines, MemoryBytes *float64 }`
- `internal/domain/topology.go` — helper `IsKnownService(id string) bool` (valida o `id` contra `KnownServices`) — ou expor um set; usado pelo service para o 404
- `internal/service/admin_service.go` — método `ServiceMetrics(ctx, id) (domain.ServiceMetrics, error)`: valida `id` (senão `ErrUnknownService`), roda as 6 queries em `errgroup`, monta o resultado (cada `scalar` reusado do 001 devolve `*float64`)
- `internal/domain/errors.go` — `ErrUnknownService` (para o transport mapear `404`)
- `internal/service/admin_service_test.go` — casos: REST conhecido (up+RED+runtime), gRPC (RED nil), métrica ausente (nil), id desconhecido (ErrUnknownService), Prometheus fora (ErrMetricsUnavailable)
- `internal/transport/http_handler.go` — rota `GET /api/services/{id}/metrics`; mapear `ErrUnknownService`->404, `ErrMetricsUnavailable`->503
- `internal/transport/http_handler_test.go` — 200 (REST), 200 com RED null (gRPC), 404 (id desconhecido), 503 (fonte fora)
- `api/README.md` + `README.md` — documentar o novo endpoint

> Sem mudança em `cmd/server/main.go` (o handler já é registrado), no Dockerfile, no compose nem no Prometheus.

## Compute e banco (ver ADR-0005)
- Sem mudança: o admin-bff segue stateless, sem banco. Só mais queries ao Prometheus.

## Dependências e libs
- Nenhuma nova. Reusa `MetricsSource`, `errgroup` e o helper `scalar` do 001.

## Decisões técnicas e trade-offs
- **`scalar` por query + `errgroup`** (mesmo padrão do `LiveMetrics`): 6 queries independentes, paralelizadas, primeiro erro cancela o resto. Escrevem campos distintos de `ServiceMetrics` (seguro sob `-race`).
- **Validar o `id` contra `KnownServices`** (404 cedo): evita montar PromQL com um `id` arbitrário do path e devolve erro claro ao front. Os ids conhecidos já existem no domínio (topologia).
- **RED `null` para gRPC** (decisão da spec): não inventar zero; o front mostra "—" / esconde RED nesses domínios. Os campos `*float64` carregam essa semântica naturalmente.
- **Erro de fonte = 503** (igual ao `/api/metrics/live`): consistência — a página de domínio degrada do mesmo jeito que o painel ao vivo.
- **PromQL como constantes no service** com o `id` interpolado: `id` é validado contra a allowlist `KnownServices` antes, então não há injeção de label arbitrário.

## Ordem de implementação
1. `domain`: tipo `ServiceMetrics`, `ErrUnknownService`, helper `IsKnownService`
2. `service`: `ServiceMetrics(ctx, id)` (errgroup das 6 queries) + unit tests (`-race`)
3. `transport`: rota + mapeamento de erro (404/503) + handler tests
4. docs (api/README, README)
5. `go test ./... -race` + `go build ./...`; validar local com um Prometheus fake (REST com RED; gRPC sem RED; id inválido -> 404; fonte fora -> 503)

## Riscos e pontos de atenção
- **Label correto:** RED usa `service="<id>"`; up/runtime usam `job="<id>"`. Confirmar que coincidem (coincidem no projeto: ex. `identity-service`). Cobrir no teste com os dois labels.
- **`errorRate` com denominador zero:** sem tráfego, `rate(...total)`=0 e a divisão vira `NaN`/vazio no Prometheus — tratar como `null` (sem amostra), não erro.
- **p95 sem amostras:** `histogram_quantile` sobre vetor vazio retorna vazio -> `null`. Ok.
- **Concorrência:** `-race` no service (6 goroutines escrevendo campos distintos).
