# Plan: 001-matchmaking-core
**Spec:** specs/001-matchmaking-core/spec.md
**Data:** 2026-06-28

## Abordagem técnica

Primeiro serviço **gRPC** (ADR-0003), hexagonal (ADR-0002). `proto/` define o contrato; o código é gerado por **buf**. O `domain` define `Match` e a Port `Matchmaker`; o `service` é a fila concorrente (o "core gordo"); o `transport` é o gRPC server (server-streaming) + interceptor de auth. Não há storage externo: a fila vive em memória (`sync.Mutex`), e cada ticket espera o resultado num **channel**.

Dois listeners no mesmo processo: **gRPC** (FindMatch) e um **HTTP** pequeno só para `/healthz` e `/metrics` (Prometheus scrapeia este).

## Codegen (buf)

- `proto/matchmaking/v1/matchmaking.proto` — `option go_package = ".../matchmaking-service/gen/matchmaking/v1;matchmakingv1"`.
- `buf.yaml` (v2) + `buf.gen.yaml` (plugins `protoc-gen-go`, `protoc-gen-go-grpc`, `paths=source_relative`, out `gen/`).
- `buf generate` (plugins em `~/go/bin`, já instalados) gera `gen/matchmaking/v1/*.pb.go`. **Código gerado é commitado** (build não depende do buf).

## Concorrência (o core, GO-LEARNINGS #01)

```
Join(ctx, playerID) -> cria ticket{playerID, ch chan Match (buf 1)}, enfileira (mutex);
                       goroutine: <-ctx.Done() -> remove o ticket da fila.
Run(ctx)            -> ticker; a cada tick, tryForm() sob mutex:
                       enquanto len(fila) >= matchSize: corta matchSize, cria Match,
                       envia no ch de cada ticket e fecha o ch.
```
O `mutex` serializa formação × cancelamento (sem corrida); o `ctx` do stream é a fonte de verdade do cancelamento. Um ticket por jogador (Join remove um ticket anterior do mesmo `sub`).

## Auth (interceptor gRPC)

`StreamServerInterceptor`: lê `authorization` do metadata, valida via `shared/auth.Verify`; falha -> `codes.Unauthenticated`. Injeta o `playerID` no context via um `ServerStream` embrulhado (override de `Context()`).

## Observabilidade (adaptada ao gRPC)

- **Tracing:** `grpc.StatsHandler(otelgrpc.NewServerHandler())` — spans por RPC (em vez do otelhttp).
- **Métricas:** servidor HTTP `/metrics` com **gauges/counters custom** (`matchmaking_queue_size`, `matchmaking_matches_total`) + runtime do Go. (RED de gRPC completo via interceptor Prometheus fica para depois; o que interessa aqui é o tamanho da fila e matches formados.)
- **Reflection** habilitada no gRPC server para permitir `grpcurl` sem o proto.

## Arquivos a criar ou modificar

- `services/matchmaking-service/proto/matchmaking/v1/matchmaking.proto`, `buf.yaml`, `buf.gen.yaml`
- `.../gen/matchmaking/v1/*.pb.go` (gerado, commitado)
- `.../internal/domain/matchmaking.go` — `Match` + Ports `Matchmaker`, `TokenVerifier`
- `.../internal/service/matchmaker.go` (+ test) — fila, `Join`, `Run`, `tryForm`, métricas de fila
- `.../internal/transport/grpc_server.go` (+ bufconn test) — `FindMatch`, interceptor de auth
- `.../cmd/server/main.go` — gRPC server (+ reflection + otelgrpc) + HTTP metrics + `Run` do matchmaker + graceful shutdown
- `.../Dockerfile`, `.../api/README.md` (exemplos grpcurl)
- `shared/config/config.go` — `GRPCPort` (env `GRPC_PORT`)
- `infra/docker/docker-compose.yml` — `matchmaking-service` (porta gRPC + porta HTTP de métricas)
- `infra/docker/prometheus.yml` — job `matchmaking-service`

## Dependências e libs

- `google.golang.org/grpc` + `google.golang.org/protobuf` (já no módulo) — promover a diretas
- `go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc` — tracing gRPC
- `github.com/google/uuid` — matchId
- buf + protoc-gen-go + protoc-gen-go-grpc (toolchain, já instalado via go install)

## Decisões técnicas e trade-offs

- **buf (não protoc):** compilador embutido, sem dependência do `protoc` do sistema; config versionada. Código gerado commitado para o `go build` não exigir o buf.
- **Server-streaming + cancel via context:** o stream é a sessão do ticket; fechar o stream cancela. Channel buffer 1 evita bloquear o `tryForm` se o cliente já saiu.
- **Fila em memória (mutex):** simples e didático; instância única no MVP. Escala horizontal exigiria fila compartilhada (Redis) — GO-LEARNINGS #01.
- **Dois listeners:** gRPC para o serviço, HTTP só para métricas/health (o ecossistema Prometheus é HTTP).

## Ordem de implementação

1. `proto` + `buf.yaml`/`buf.gen.yaml` + `buf generate` (validar o codegen cedo)
2. `shared/config` (GRPC_PORT)
3. `domain` (Match, Ports)
4. `service` (matchmaker concorrente) + unit tests (`-race`): formação, cancelamento, matchSize
5. `transport` (gRPC server + interceptor) + bufconn test (auth + happy path)
6. wire-up `main` (gRPC + HTTP metrics + Run) + Dockerfile + compose + job no Prometheus
7. `api/README.md` (grpcurl)
8. `go test ./... -race` verde; validar no Docker (2 clientes grpcurl -> match)

## Riscos e pontos de atenção

- **Config do buf/codegen:** validar `buf generate` antes de seguir (passo 1).
- **Corridas:** rodar o service test com `-race` (formação × cancelamento concorrentes).
- **Teste de stream:** usar `bufconn` (gRPC in-process) para não depender de porta.
- **grpcurl** pode não estar instalado para o teste manual — a validação no Docker pode usar um cliente Go simples ou grpcurl se disponível.
