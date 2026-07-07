# Tasks: 012-invadable-world-directory

Fatia H1a (lado session) do `host_world_dropin` (ADR-0023). Aditiva sobre a H0. Marcar `[x]` ao concluir. Ver `plan.md` para as decisões (D1–D5).

Baseline: `go test ./services/session-service/... -race` **verde** antes de começar.

## Domínio

- [ ] **T1 — tipos e assinaturas.** `internal/domain/session.go`: `Session` += `HostSoulLevel int`, `WeaponTier int`; novos `WorldAttributes{SoulLevel, WeaponTier int}` e `InvadableWorld{SessionID string; HostSoulLevel, WeaponTier, FreeInvaderSlots int}`. `SessionService.OpenWorld` ganha `attrs WorldAttributes`; novo `ListInvadableWorlds(ctx, gameID) ([]InvadableWorld, error)`. `JoinWorld` passa a devolver o `server_address` (D3) — ajustar a assinatura da Port (ex.: devolver `(ConnectGrant, serverAddr string, error)` ou um struct).

## Service

- [ ] **T2 — OpenWorld guarda attrs + JoinWorld devolve o addr.** `internal/service/session_service.go`: `OpenWorld` grava `HostSoulLevel`/`WeaponTier` no `Session`; `JoinWorld` retorna o `ServerAddress` da sessão junto do grant. Ajustar os testes/helpers da H0 (`openWorldSetup`, `host_world_test.go`) para as novas assinaturas.
- [ ] **T3 — ListInvadableWorlds.** Varre `s.sessions` sob o lock: só `Origin=host_world`, `GameID` igual, com slot `invader` de capacidade livre; monta `InvadableWorld` com `FreeInvaderSlots`. Exclui invasor cheio / sem papel invasor / outro jogo / `queue_arena`. Vazio → lista vazia, sem erro. Testes de cada exclusão + `freeInvaderSlots` correto.

## Proto + transport

- [ ] **T4 — proto.** `proto/session/v1/session.proto`: `OpenWorldRequest` += `int32 host_soul_level = 3; int32 weapon_tier = 4;`; `JoinWorldResponse` += `string server_address`; novo RPC `ListInvadableWorlds` + messages (`ListInvadableWorldsRequest`/`Response`/`InvadableWorld`). Regenerar: `PATH="$(go env GOPATH)/bin:$PATH" buf generate` (nunca `sed`).
- [ ] **T5 — handlers.** `internal/transport/grpc_server.go`: `OpenWorld` passa `WorldAttributes`; `JoinWorld` devolve `server_address`; novo handler `ListInvadableWorlds` (tradução pura). Teste bufconn (`host_world_grpc_test.go`): OpenWorld com attrs → ListInvadableWorlds reflete; encher o slot de invasor → mundo some da lista.

## Fechamento

- [ ] **T6 — verde + lint.** `go test ./services/session-service/... -race -cover` (≥80% service/domain). `golangci-lint run ./services/session-service/...`. Confirmar queue_arena (001–010) + H0 (011) sem regressão.
