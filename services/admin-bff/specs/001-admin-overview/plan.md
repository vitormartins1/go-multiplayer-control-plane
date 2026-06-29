# Plan: 001-admin-overview
**Spec:** specs/001-admin-overview/spec.md
**Data:** 2026-06-29

## Abordagem técnica

Novo serviço Go **REST** (`net/http`), hexagonal (ADR-0002), seguindo o esqueleto dos serviços REST existentes (identity/achievements). É um **agregador read-only** (ADR-0016): o `domain` define os tipos + duas Ports de saída — `MetricsSource` (Prometheus) e `AchievementsReader` (API admin do achievements); o `service` orquestra; os adapters falam HTTP com o Prometheus e com o achievements. **Sem banco** (não persiste nada).

A topologia tem **nós** = os serviços Go (derivados de uma lista estática, já que sabemos quais são) com `up` vindo de `up{job=...}` do Prometheus, e **arestas** = dependências conhecidas (estáticas no MVP). Os contadores ao vivo são `instant queries` ao Prometheus. Onde houver várias consultas independentes (ex.: montar a topologia + métricas), usamos **`errgroup`** para o fan-out concorrente (o exercício de Go desta feature).

Degradação por fonte (decisão da spec): Prometheus fora -> topologia retorna nós `unknown` + `sourceAvailable=false` (200); `/api/metrics/live` -> 503; achievements fora -> `/api/players/{id}/achievements` -> 502.

## Arquivos a criar ou modificar

- `internal/domain/admin.go` — tipos (`ServiceNode`, `Edge`, `Topology`, `LiveMetrics`, `PlayerAchievement`, `Sample`); Ports `MetricsSource` (`Query(ctx, promQL) ([]Sample, error)`) e `AchievementsReader` (`ForPlayer(ctx, playerID) ([]PlayerAchievement, error)`); a topologia conhecida (nós + arestas) como dados estáticos
- `internal/service/admin_service.go` (+ test) — `Topology(ctx)` (query `up{}` + monta nós/arestas, com `errgroup` se houver mais de uma query), `LiveMetrics(ctx)` (queries dos contadores em paralelo via `errgroup`), `PlayerAchievements(ctx, id)` (delega ao reader); degradação por fonte
- `internal/prometheus/client.go` (+ test) — adapter `MetricsSource`: `GET {PROMETHEUS_URL}/api/v1/query?query=...`, parseia o JSON (`data.result[].metric` + `value`); `httptest` no teste
- `internal/achievements/client.go` (+ test) — adapter `AchievementsReader`: `GET {ACHIEVEMENTS_URL}/admin/players/{id}/achievements`, mapeia o JSON para `[]PlayerAchievement`; `httptest` no teste
- `internal/transport/http_handler.go` (+ test) — rotas `/api/topology`, `/api/metrics/live`, `/api/players/{playerID}/achievements`; mapeia erros de fonte para 200(degrada)/503/502; `httptest`
- `cmd/server/main.go` — wire-up: config (`PROMETHEUS_URL`, `ACHIEVEMENTS_URL`), clients HTTP com `otelhttp` (trace cross-service), `middleware.NewMetrics` + `RouteSpanName` (RED + OpenMetrics), `/healthz`, `/metrics`, graceful shutdown
- `services/admin-bff/Dockerfile`, `api/README.md` (curl), `README.md`
- `infra/docker/docker-compose.yml` — serviço `admin-bff` (HTTP, `PROMETHEUS_URL=http://prometheus:9090`, `ACHIEVEMENTS_URL=http://achievements-service:8080`, `depends_on` prometheus/achievements); `infra/docker/prometheus.yml` — job `admin-bff`

## Compute e banco (ver ADR-0005)
- **Sem banco** (agregador stateless). Compute fica diferido (ADR-0005) — provável Fargate/Lambda atrás de um gateway; decidido na fase de deploy. Localmente é só mais um container.

## Dependências e libs
- `net/http` + `encoding/json` (stdlib) para os adapters — **sem** a client lib do Prometheus (raw HTTP na `/api/v1/query` mantém deps mínimas e é didático)
- `golang.org/x/sync/errgroup` (já no módulo como indirect; vira direct) — fan-out concorrente
- `shared/middleware`, `shared/logger`, `shared/telemetry`, `otelhttp` — RED + tracing, como os demais

## Decisões técnicas e trade-offs
- **Raw HTTP na HTTP API do Prometheus** (vs `github.com/prometheus/client_golang/api`): menos dependência e mais aprendizado (parsear `/api/v1/query` na mão). Trade-off: nós mesmos lidamos com o shape do JSON; aceitável porque usamos poucas queries simples (instant).
- **Topologia estática** (nós e arestas conhecidos em código) no MVP: sabemos quem chama quem (matchmaking→session/identity, achievements→identity, admin-bff→prometheus/achievements). Trade-off: arestas não são "descobertas" automaticamente — evolução poderia derivá-las de métricas/trace, mas é overkill agora.
- **`errgroup` para o fan-out**: as queries ao Prometheus e a montagem dos painéis são independentes — rodar em paralelo com cancelamento por context é idiomático (≈ `Task.WhenAll` do .NET, mas com `ctx`). É o conceito de Go novo desta feature (a registrar no GO-LEARNINGS).
- **Degradação isolada por fonte** (decisão da spec): o service distingue "fonte indisponível" de "erro de dados" e o transport mapeia para 200-degradado/503/502 conforme o painel. Mantém o painel útil mesmo com o Prometheus oscilando.
- **`otelhttp` nos clients de saída**: propaga o trace do BFF até o achievements (e o contexto às chamadas ao Prometheus), fechando a visão cross-service (ADR-0014).
- **Sem auth** (decisão confirmada): API do BFF e endpoints admin internos/local; auth real é ADR futuro.

## Ordem de implementação
1. `domain` (tipos + Ports + topologia estática)
2. `prometheus` client + test (`httptest`: parse de `up{}` e de um contador)
3. `achievements` client + test (`httptest`: mapear o JSON admin)
4. `service` (Topology/LiveMetrics/PlayerAchievements + degradação) + unit tests (fakes das Ports)
5. `transport` (rotas + mapeamento de erro→status) + handler tests
6. wire-up `main` + Dockerfile + compose + job Prometheus + READMEs
7. `go test ./... -race` + `go build ./...`; validar local: `/api/topology`, `/api/metrics/live`, `/api/players/{id}/achievements`

## Riscos e pontos de atenção
- **Shape do JSON do Prometheus**: `/api/v1/query` devolve `{status, data:{resultType, result:[{metric:{...}, value:[ts, "v"]}]}}` — o valor é **string**; parsear com cuidado e cobrir no teste.
- **`up{}` ausente**: um serviço que o Prometheus nunca raspou não aparece em `up` — tratar como `down`/`unknown` conforme a disponibilidade da fonte (não confundir "serviço down" com "Prometheus down").
- **Dependência de ordem**: o painel de achievements depende da feature `achievements/003-admin-read` estar implementada. Implementar a 003 antes (ou o painel degrada/erra até existir).
- **Concorrência**: testar o service com `-race` (fan-out com `errgroup`).
- **CORS**: o `admin-ui` (front separado) vai chamar o BFF do browser — pode precisar de header CORS no BFF. Anotar; tratar quando o front existir (fora deste MVP de backend, mas a rota pode já nascer com um middleware simples se necessário).
