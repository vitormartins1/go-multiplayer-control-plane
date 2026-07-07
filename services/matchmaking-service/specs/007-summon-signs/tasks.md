# Tasks: 007-summon-signs

Fatia H1b (co-op por quadro de sinais) do `host_world_dropin` (ADR-0023). O terceiro modo do matchmaking: anúncio → escolha do host → injeção `coop`. Ver `plan.md` (D1–D7). Marcar `[x]` ao concluir.

**Pré-requisito:** H1a (`matchmaking/006`) mergeada — reusa `WorldInjector`, o registryclient e o padrão de elegibilidade pura. Baseline `go test ./services/matchmaking-service/... -race` **verde** antes.

## Domínio

- [x] **T1 — tipos, Ports, erros.** `internal/domain/coop.go` (novo): `Sign{SignID, GameID, PlayerID string; SoulLevel, WeaponTier int; PostedAt time.Time}`, `Summoned{SessionID, ServerAddress, ConnectToken string}`, `CoopCriteria{SoulLevel, WeaponTier int}`, `CoopPolicy{Archetype string; CoopEnabled, LevelRangeEnabled, WeaponTierEnabled bool}` (+ `SupportsCoop()`). Ports `Summoner` (PostSign/ListSigns/Summon/CancelSign) e `CoopPolicyProvider`. `errors.go`: `ErrCoopNotSupported`, `ErrSignNotFound`, `ErrSignConsumed`. Função pura `CoopEligible(ally CoopCriteria, hostSoulLevel, hostWeaponTier int, p CoopPolicy) bool` (banda **simétrica** `A ± (coopFlat + coopFraction·A)`, consts tunáveis; nível 0 + range on → false).
- [x] **T1b — teste de CoopEligible** (tabela): banda simétrica (A=100 → 80 e 120 sim; 79 e 121 não), `LevelRangeEnabled=false` casa qualquer, weapon tier on/off, nível 0 + range on → false.

## Adapter de saída

- [x] **T2 — registryclient.CoopPolicy.** `internal/registryclient/client.go`: `enabledRoles []string` no parse; método `CoopPolicy(ctx, gameID)` → `CoopEnabled` = contém `"coop"` + flags de range/tier. Reusa `getProfile`. 404/rede → `ErrProfileUnavailable`. Teste httptest (enabledRoles com/sem coop; arquétipo errado → `!SupportsCoop`).

## Serviço

- [x] **T3 — Summoner.** `internal/service/summoner.go` (novo): store interno `signs map[string]*sign` (sign = `domain.Sign` + `ch chan domain.Summoned` bufer 1 + `claimed bool`) sob mutex; `PostSign` (policy→SupportsCoop; publica active + canal), `ListSigns` (filtra active por `CoopEligible`), `Summon` (claim atômico → inject `coop` fora do lock → entrega no canal + consome; `ErrRoleFull`/`ErrWorldNotFound` → release, sinal segue active), `CancelSign` (remove+close se não claimed), `SweepExpired(ttl)` + `RunSweeper`. Contadores `summons_total{summoned|expired|cancelled}` via `SummonsTotal`; `SignsActive()`. Testes com mocks de policy+injector (todos os ramos, inclusive corrida de claim e role-full sem consumir).

## Proto + transport

- [x] **T4 — proto.** `proto/matchmaking/v1/matchmaking.proto`: `PostSummonSign(...) returns (stream SummonUpdate)`, `ListSigns(...)` unary, `SummonSign(...)` unary + messages + `enum SummonStatus{UNSPECIFIED, POSTED, SUMMONED, EXPIRED}`. `buf generate`.
- [x] **T5 — handlers.** `internal/transport/grpc_server.go`: `NewServer(mm, inv, sum)`; `PostSummonSign` (stream: POSTED → select ctx.Done/canal, SUMMONED vs EXPIRED por `ok`); `ListSigns`/`SummonSign` (unary; playerID do ctx). Mapa de erros → codes. Testes bufconn: POSTED; SummonSign entrega SUMMONED no stream do aliado; ListSigns reflete e some após invocar; erros → codes.

## Wire-up + fechamento

- [x] **T6 — main.go.** Constrói o `Summoner` (coop policy = registryclient cru; injector = sessionclient); `RunSweeper` do summoner (goroutine); `summons_total{result}` (3× CounterFunc) + `summon_signs_active` gauge; `NewServer(mm, inv, sum)`.
- [x] **T7 — verde + lint + docs.** `go test ./services/matchmaking-service/... -race -cover` (≥80% service/domain). `golangci-lint run`. `FindMatch`/`Invade` sem regressão. GO-LEARNINGS se surgir algo (candidato: `ok` do receive como discriminador SUMMONED/EXPIRED; claim/release reversível). Atualizar a tabela de estado do README de game-models (H1b).
