# Tasks: 006-world-discovery-invade

Fatia H1a (lado matchmaking) do `host_world_dropin` (ADR-0023). O segundo modo do matchmaking: descobrir + injetar. Ver `plan.md` (D1–D6). Marcar `[x]` ao concluir.

**Pré-requisito:** `session/012` implementado e mergeado (consome os tipos gRPC do session, incluindo `server_address` no `JoinWorldResponse`). Baseline `go test ./services/matchmaking-service/... -race` **verde** antes.

## Domínio

- [x] **T1 — tipos, Ports, erros, Eligible.** `internal/domain/matchmaking.go`: `InvaderCriteria{SoulLevel, WeaponTier int}`, `InvadableWorld{SessionID; HostSoulLevel, WeaponTier, FreeInvaderSlots int}`, `Injected{ServerAddress, ConnectToken string}`, `Invaded{ServerAddress, ConnectToken, SessionID string}`, `InvasionPolicy{Archetype string; InvasionEnabled, LevelRangeEnabled, WeaponTierEnabled bool}`. Ports: `Invader`, `InvasionPolicyProvider`, `WorldDirectory`, `WorldInjector`. `errors.go`: `ErrInvasionNotSupported`, `ErrNoEligibleWorld`. Função pura `Eligible(crit, world, policy) bool` (banda assimétrica, constantes tunáveis; host sem atributos + range on → não elegível).
- [x] **T1b — teste de Eligible** (tabela): dentro/fora da banda, assimetria (L=100 → 90 e 130 sim; 89 e 131 não), `LevelRangeEnabled=false` casa qualquer, weapon tier on/off, host attrs zerados.

## Adapters de saída

- [x] **T2 — registryclient.InvasionPolicy.** `internal/registryclient/client.go`: método novo lendo `GET /games/{id}/multiplayer-profile` → `InvasionPolicy` (`archetype` + `invasionEnabled`/`levelRangeEnabled`/`weaponTierEnabled`). 404/rede → `ErrProfileUnavailable`. Teste httptest.
- [x] **T3 — sessionclient.** `internal/sessionclient/grpc_client.go`: `ListInvadableWorlds(ctx, gameID)` (traduz `[]*sessionv1.InvadableWorld`) e `Inject(ctx, sessionID, playerID, role)` (chama `JoinWorld`, usa o `server_address` da resposta; mapeia `ResourceExhausted`→`ErrRoleFull`, `NotFound`→`ErrWorldNotFound`). Testes.

## Serviço

- [x] **T4 — Invader.** `internal/service/invader.go` (novo): `Invade(ctx, gameID, playerID, crit)` orquestra policy → checa arquétipo/`InvasionEnabled` (`ErrInvasionNotSupported`) → `ListInvadableWorlds` → filtra `Eligible` na ordem → injeta o 1º; `ErrRoleFull` tenta o próximo; esgotados/vazio → `ErrNoEligibleWorld`. Timeout nas chamadas ao session. Contadores `invasions_total{injected|no_world|not_supported}` via `InvasionsTotal(result)`. Testes com mocks das 3 Ports (todos os ramos, inclusive retry).

## Proto + transport

- [x] **T5 — proto.** `proto/matchmaking/v1/matchmaking.proto`: RPC unary `Invade(InvadeRequest{game_id, int32 soul_level, int32 weapon_tier}) → InvadeResponse{session_id, server_address, connect_token}`. `buf generate`.
- [x] **T6 — handler + interceptor unary.** `internal/transport/grpc_server.go`: `UnaryAuthInterceptor` (espelho do stream: bearer → verify → `playerIDKey` no ctx); handler `Invade` (playerID do ctx, valida game_id, mapeia erros → `FailedPrecondition`/`NotFound`/`Internal`). `Server` passa a segurar `domain.Invader` (`NewServer(mm, inv)`). Testes de mapeamento + token ausente → `Unauthenticated`.

## Wire-up + fechamento

- [x] **T7 — main.go.** Constrói o `Invader` (policy = registryclient cru; directory/injector = sessionclient sobre o `sessionv1.SessionClient` já conectado); registra `grpc.ChainUnaryInterceptor(UnaryAuthInterceptor(verifier))`; 3× `CounterFunc` `invasions_total{result}`.
- [x] **T8 — verde + lint + docs.** `go test ./services/matchmaking-service/... -race -cover` (≥80% service/domain). `golangci-lint run`. `FindMatch` sem regressão. GO-LEARNINGS se surgir algo (candidato: orquestração stateless list→filter→inject com retry; struct satisfazendo múltiplas Ports). Atualizar a tabela de estado do `README` de game-models (H1a).
