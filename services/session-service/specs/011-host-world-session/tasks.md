# Tasks: 011-host-world-session

Fatia H0 do `host_world_dropin` (ADR-0023). Cada task é atômica e verificável. Marcar `[x]` ao concluir. Ordem = dependência (domínio → adapters → service → transport → wire-up).

Baseline: `go test ./services/session-service/... -race` **verde** antes de começar (57 testes).

## Domínio

- [x] **T1 — tipos e Ports.** Em `internal/domain/session.go`: `SessionOrigin` (`OriginMatch`/`OriginHostWorld`), consts de papel (`RoleHost`/`RoleCoop`/`RoleInvader`), `RoleSlot{Capacity, Occupants}`, campos novos em `Session` (`Origin`, `HostPlayerID`, `RoleSlots`), `CloseHostLeft`. Nova assinatura `ConnectTokenIssuer.Issue(playerID, refID, serverAddr string, claims ConnectClaims)` + `ConnectClaims{TeamID, Role}`. Novos métodos em `SessionService` (`OpenWorld`/`JoinWorld`/`LeaveWorld`/`CloseWorld`). Port `ProfileProvider` + `WorldProfile`.
- [x] **T2 — erros.** Em `internal/domain/errors.go`: `ErrUnsupportedArchetype`, `ErrRoleNotEnabled`, `ErrRoleFull`, `ErrWorldNotFound`, `ErrAlreadyJoined`, `ErrProfileUnavailable`.

## Adapters de saída

- [x] **T3 — token issuer.** `internal/token/connect_issuer.go`: `Issue` com `ConnectClaims` (claim `role` quando não-vazio; `team_id` mantido). Ajustar o call site do `Create` no service. Testes: `role` presente/ausente, `team_id` intacto (não regride queue_arena).
- [x] **T4 — registryclient (perfil host_world).** Novo `internal/registryclient/client.go`: `GET {GAME_REGISTRY_URL}/games/{id}/multiplayer-profile` → `WorldProfile`. Arquétipo ≠ `host_world_dropin`, status ≠200, rede → `ErrProfileUnavailable`. Tags: `enabledRoles`/`coopSlots`/`invaderSlots`. Testes espelhando o do matchmaking (httptest).

## Service (núcleo)

- [x] **T5 — D1: rechaveação por sessionID.** `Service.sessions` passa a ser `map[sessionID]*Session`; novo `matchIndex map[matchID]sessionID`. Ajustar `Create` (indexar), `ReportResult`/`CloseByAdmin` (resolver via matchIndex), `closeSession` (opera por sessionID + limpa matchIndex). `Sweep`/`reconcile` mudam só o alvo do `delete`. **Rodar os testes de 001–010 verdes — nenhuma regressão.**
- [x] **T6 — OpenWorld.** Lê perfil (`profiles.WorldProfile`) → arquétipo errado/indisponível = `ErrUnsupportedArchetype`; aloca (`ErrNoServer`); emite token host (`ConnectClaims{Role: RoleHost}`, rollback do server no erro); monta `RoleSlots` do perfil; registra `Origin=host_world`. Testes com mocks.
- [x] **T7 — JoinWorld / LeaveWorld / CloseWorld.** Join: `ErrWorldNotFound`/`ErrRoleNotEnabled`/`ErrAlreadyJoined`/`ErrRoleFull`; ocupa slot; token fora do lock com reversão no erro. Leave: libera slot, no-op idempotente, host → CloseWorld. Close: `closeSession(sessionID, CloseHostLeft)`. `OccupiedSlots(role)` para a métrica. Testes de cada ramo.

## Transport + proto

- [x] **T8 — proto.** `proto/session/v1/session.proto`: 4 RPCs + messages (spec §"Protobuf"). Regenerar: `PATH="$(go env GOPATH)/bin:$PATH" buf generate` (nunca `sed`).
- [x] **T9 — handlers gRPC.** `internal/transport/grpc_server.go`: 4 handlers, tradução pura + mapa de erros → codes (`FailedPrecondition`/`InvalidArgument`/`ResourceExhausted`/`NotFound`/`Internal`). Testes de mapeamento.

## Wire-up + observabilidade

- [x] **T10 — main.go.** Montar `ProfileProvider` (http.Client com `auth.InternalKeyTransport` + `otelhttp`, `GAME_REGISTRY_URL`) e passar ao `service.New`. Counter `sessions_closed_total{reason="host_left"}`. Gauge `world_role_slots{role}` via `svc.OccupiedSlots`.

## Fechamento

- [x] **T11 — verde total + validação.** `go test ./services/session-service/... -race -cover` (≥80% em service/domain). `golangci-lint run`. Validação `grpcurl`: OpenWorld → JoinWorld(coop) → JoinWorld(invader) → JoinWorld além da capacidade (ResourceExhausted) → LeaveWorld → CloseWorld; conferir `world_role_slots` e `host_left`.
- [x] **T12 — GO-LEARNINGS + docs.** Registrar aprendizado novo (ex.: dois arquétipos no mesmo agregado de sessão; rechaveação sem regressão). Atualizar a tabela de estado do `README` (host_world comportamento H0 → 🔵 branch).
