# Plan: 001-session-core
**Spec:** specs/001-session-core/spec.md
**Data:** 2026-06-28

## Abordagem técnica

Segundo serviço **gRPC** (ADR-0003), hexagonal (ADR-0002). `proto/` define o contrato (codegen com **buf**, mesmo padrão do matchmaking); `domain` define os tipos + Ports; `service` orquestra (estado em memória); adapters: `allocator` (mock), `token` (connect token Ed25519), `publisher` (fan-out stub). Dois listeners: **gRPC** (CreateSession, ReportResult, unary) + **HTTP** (JWKS do connect token, `/healthz`, `/metrics`).

É **control plane interno** — sem JWT de jogador no MVP (o cliente fala com o matchmaking). Auth serviço-a-serviço é evolução.

## Arquivos a criar ou modificar

- `proto/session/v1/session.proto` + `buf.yaml` + `buf.gen.yaml` + `gen/session/v1/*.pb.go` (gerado, commitado)
- `internal/domain/session.go` — `Session`, `Server`, `ConnectGrant`, `PlayerResult` + Ports `SessionService`, `ServerAllocator`, `ConnectTokenIssuer`, `ResultPublisher`
- `internal/domain/errors.go` — `ErrNoServer`, `ErrSessionNotFound`
- `internal/token/connect_issuer.go` (+ test) — JWT EdDSA (claims `sub`/`match_id`/`server`/`exp`) + JWKS; reusa o padrão do identity (`golang-jwt`, Ed25519, JWKS à mão)
- `internal/allocator/static.go` (+ test) — pool fixo em memória, alocação atômica (`sync.Mutex`); `ErrNoServer` ao esgotar
- `internal/service/session_service.go` (+ test) — `Create` (aloca + emite tokens + registra), `ReportResult` (publica + libera + encerra); sessões em `map` sob mutex
- `internal/publisher/log_publisher.go` — `ResultPublisher` stub que loga (fan-out real = próxima feature)
- `internal/transport/grpc_server.go` (+ bufconn test) — `CreateSession`, `ReportResult`
- `cmd/server/main.go` — gRPC (otelgrpc + reflection) + HTTP (JWKS/metrics/healthz) + graceful shutdown; chave Ed25519 (efêmera em dev, como o identity)
- `services/session-service/Dockerfile`, `api/README.md` (grpcurl)
- `infra/docker/docker-compose.yml` — `session-service` (gRPC + HTTP), `infra/docker/prometheus.yml` — job

## Decisões técnicas e trade-offs

- **Connect token = chave Ed25519 própria do session** (não a do identity): cunhar o token de partida é responsabilidade do session. Expõe `/.well-known/jwks.json` próprio; o Unity DS valida **offline** (GO-LEARNINGS #02). Em dev, chave efêmera gerada no boot (com aviso), igual ao identity.
- **Allocator mock atrás de Port** (`ServerAllocator`): pool de `Server{ID,Address}` fixo, `Allocate`/`Release` atômicos sob mutex (GO-LEARNINGS #01). O adapter Agones real (Allocation API) entra no deploy (ADR-0011/0012) — só troca o adapter.
- **Fan-out via Port `ResultPublisher`, adapter stub (loga)** no MVP. O fan-out HTTP real precisa de endpoints server-side no leaderboard/achievements + **service-to-service auth** (provável ADR) — fica para a próxima feature.
- **gRPC unary** (sem streaming no MVP) + reflection; HTTP separado só para JWKS/metrics (como o matchmaking).
- **Estado em memória** (sessões + pool): efêmero; prod persistiria (DynamoDB/Redis).

## Ordem de implementação

1. `proto` + buf + `buf generate` (validar codegen — mesmo padrão do matchmaking)
2. `domain` (tipos, Ports, errors)
3. `token` (connect issuer Ed25519 + JWKS) + test (assinatura verificável; claims)
4. `allocator` (mock) + test (`-race`): aloca único, esgota → ErrNoServer, release reabre
5. `service` (Create/ReportResult) + unit tests (mocks de allocator/token/publisher)
6. `publisher` stub
7. `transport` (gRPC server) + bufconn test
8. wire-up `main` + Dockerfile + compose + job; validar no Docker (CreateSession → token verificável no JWKS; ReportResult libera server)

## Riscos e pontos de atenção

- **Codegen buf:** já validado no matchmaking; replicar a config.
- **Concorrência do allocator/sessões:** testar com `-race`.
- **Claims do connect token:** garantir `sub`/`match_id`/`server`/`exp` e verificabilidade via JWKS.
- **grpcurl** ausente local — validação no Docker via cliente Go temporário (como no matchmaking).
