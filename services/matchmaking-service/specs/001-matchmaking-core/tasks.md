# Tasks: 001-matchmaking-core
**Plan:** specs/001-matchmaking-core/plan.md
**Data:** 2026-06-28

> Ordem: proto/codegen → config → domain → service(concorrência) → transport(gRPC) → wire-up/infra → collection.
> Testes acompanham cada camada (service com -race). Marcar [x] conforme implementa.

## Fase 1 — Proto e codegen (buf)
- [x] `proto/matchmaking/v1/matchmaking.proto` — service `Matchmaking`, `FindMatch` (server-streaming), `TicketUpdate`, `Status`
- [x] `buf.yaml` + `buf.gen.yaml` (plugins protoc-gen-go / protoc-gen-go-grpc, out `gen/`)
- [x] `buf generate` -> `gen/matchmaking/v1/*.pb.go` (commitado); `go build` do pacote gerado

## Fase 2 — Base compartilhada
- [x] `shared/config/config.go` — `GRPCPort` (env `GRPC_PORT`)

## Fase 3 — Domain
- [x] `internal/domain/matchmaking.go` — `Match{ID, Players, ConnectInfo}` + Ports `Matchmaker`, `TokenVerifier`

## Fase 4 — Service (matchmaker concorrente)
- [x] `internal/service/matchmaker.go` — fila (`sync.Mutex`), `Join` (ticket+channel, remove dup, cancel via ctx), `Run` (ticker), `tryForm` (FIFO matchSize), métricas de fila
- [x] `internal/service/matchmaker_test.go` — unit `-race`: matchSize jogadores → match com mesmo ID; cancelado sai da fila; < matchSize não forma; dup do mesmo player

## Fase 5 — Transport (gRPC)
- [x] `internal/transport/grpc_server.go` — `FindMatch` (Join → SEARCHING → MATCHED/cancel) + `StreamServerInterceptor` de auth (metadata → shared/auth → playerID no ctx)
- [x] `internal/transport/grpc_server_test.go` — bufconn: sem token → Unauthenticated; 2 clientes → MATCHED

## Fase 6 — Wire-up, infra e verificação
- [x] `cmd/server/main.go` — gRPC server (otelgrpc StatsHandler + reflection + interceptor) + HTTP `/metrics` `/healthz` + `Run` do matchmaker + graceful shutdown (ambos listeners)
- [x] `services/matchmaking-service/Dockerfile`
- [x] `infra/docker/docker-compose.yml` — `matchmaking-service` (porta gRPC + porta HTTP métricas; env GRPC_PORT/JWKS/OTEL)
- [x] `infra/docker/prometheus.yml` — job `matchmaking-service`
- [x] `go test ./... -race` verde; `go build ./...`
- [x] Validar no Docker: 2 clientes (grpcurl/cliente Go) → match; sem token → Unauthenticated; métrica de fila no Prometheus

## Fase 7 — Collection
- [x] `services/matchmaking-service/api/README.md` — exemplos grpcurl (login no identity → token → FindMatch)
