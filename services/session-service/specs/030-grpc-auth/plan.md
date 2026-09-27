# Plan: 030-grpc-auth

## Decisões técnicas

- **D1 — interceptor unário no transport** (`internal/transport/auth.go`): identifica o chamador a partir da
  metadata e aplica a tabela RPC → tipos aceitos. Não há RPC de streaming no session. O chamador vai para o `ctx`
  (`callerFrom(ctx)`) para os handlers que precisam dele (`OpenWorld`, `LeaveWorld`, `CloseWorld`).
- **D2 — regra "é o host?" no service** (`IsWorldHost`, `JoinWorld(..., summonerID)`): a identificação é
  transport, a regra de negócio sobre o mundo é service (ADR-0002).
- **D3 — verificador de JWT reaproveitado**: `shared/auth.Verifier` via uma Port mínima no transport
  (`PlayerVerifier`), com fake nos testes.
- **D4 — chaves comparadas em tempo constante** (`crypto/subtle`), chave vazia nunca valida (fail closed, mesmo
  critério do `RequireInternalKey`).
- **D5 — métrica** `session_auth_rejections_total{method,reason}` (CounterVec criado no main e injetado).
- **D6 — matchmaking**: `grpc.WithPerRPCCredentials` com a internal key (transporte inseguro permitido — rede
  interna, igual ao HTTP da ADR-0018).

## Arquivos

- `proto/session/v1/session.proto` (+ `buf generate`): `JoinWorldRequest.summoner_player_id = 4`.
- `internal/domain/errors`: `ErrNotWorldHost`.
- `internal/service/session_service.go`: `JoinWorld(..., summonerID)`, `IsWorldHost`.
- `internal/transport/auth.go` (+ teste), `grpc_server.go` (usa o chamador; mapeia `ErrNotWorldHost`).
- `cmd/server/main.go`: verifier, chaves, modo, métrica, interceptor.
- matchmaking: `sessionclient` (credenciais + `summoner`), `domain.WorldInjector.Inject`, `service/summoner.go`,
  `transport/grpc_server.go` (`PermissionDenied`).
- infra: compose (session e matchmaking), `infra/agones/fleets/*.yaml`, `scripts/demo-host-world.sh`.
