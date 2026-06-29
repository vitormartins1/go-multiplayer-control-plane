# Tasks: 001-session-core
**Plan:** specs/001-session-core/plan.md
**Data:** 2026-06-28

> Ordem: proto/codegen → domain → token → allocator → service → publisher → transport(gRPC) → wire-up/infra.
> Testes acompanham cada camada (allocator/service com -race). Marcar [x] conforme implementa.

## Fase 1 — Proto e codegen (buf)
- [x] `proto/session/v1/session.proto` — `Session` service (`CreateSession`, `ReportResult`), mensagens (`ConnectGrant`, `PlayerResult`)
- [x] `buf.yaml` + `buf.gen.yaml` + `buf generate` → `gen/session/v1/*.pb.go` (commitado); `go build` do gerado

## Fase 2 — Domain
- [x] `internal/domain/session.go` — `Session`, `Server`, `ConnectGrant`, `PlayerResult` + Ports `SessionService`, `ServerAllocator`, `ConnectTokenIssuer`, `ResultPublisher`
- [x] `internal/domain/errors.go` — `ErrNoServer`, `ErrSessionNotFound`

## Fase 3 — Connect token (Ed25519)
- [x] `internal/token/connect_issuer.go` — `Issue(playerID, matchID, server)` → JWT EdDSA (`sub`/`match_id`/`server`/`exp`); `JWKS()`; chave própria (efêmera em dev)
- [x] `internal/token/connect_issuer_test.go` — token verificável com a pública; claims corretos; expira

## Fase 4 — Allocator (mock)
- [x] `internal/allocator/static.go` — pool fixo em memória; `Allocate`/`Release` atômicos (`sync.Mutex`); `ErrNoServer` ao esgotar
- [x] `internal/allocator/static_test.go` — `-race`: aloca server único; esgota → `ErrNoServer`; release reabre vaga

## Fase 5 — Service
- [x] `internal/service/session_service.go` — `Create` (aloca → emite tokens por player → registra sessão); `ReportResult` (publica → libera server → encerra); sessões em `map` sob mutex
- [x] `internal/service/session_service_test.go` — unit (mocks): create gera N grants + serverAddress; pool esgotado → ErrNoServer; ReportResult publica+libera; matchId inexistente → ErrSessionNotFound

## Fase 6 — Publisher (stub)
- [x] `internal/publisher/log_publisher.go` — `ResultPublisher` que loga o resultado (fan-out real = próxima feature)

## Fase 7 — Transport (gRPC)
- [x] `internal/transport/grpc_server.go` — `CreateSession`, `ReportResult` (mapeia erros → codes)
- [x] `internal/transport/grpc_server_test.go` — bufconn: CreateSession retorna grants/server; ReportResult ok; erros → ResourceExhausted/NotFound

## Fase 8 — Wire-up, infra e verificação
- [x] `cmd/server/main.go` — gRPC (otelgrpc + reflection) + HTTP (`/.well-known/jwks.json`, `/metrics`, `/healthz`) + graceful shutdown; chave Ed25519 efêmera em dev
- [x] `services/session-service/Dockerfile` + `api/README.md` (grpcurl)
- [x] `infra/docker/docker-compose.yml` — `session-service` (gRPC + HTTP) + `infra/docker/prometheus.yml` — job
- [x] `go test ./... -race` verde; `go build ./...`
- [x] Validar no Docker: CreateSession → connect token verificável no JWKS; ReportResult libera o server; métricas no Prometheus
