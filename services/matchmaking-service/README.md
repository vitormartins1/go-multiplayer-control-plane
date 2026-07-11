# matchmaking-service

≈ **GameLift FlexMatch**. **Primeiro serviço gRPC** (ADR-0003). Serve dois modelos de entrada num mundo:

- **Partida simétrica** (`queue_arena` e afins): o jogador entra na fila e acompanha o ticket por **server-streaming** até a partida formar (FIFO por tamanho de time). É o "core de concorrência" do backend (fila + worker + channels).
- **Invasão e sinais de co-op** (`host_world_dropin`, ADR-0023): não há fila nem partida — o matchmaking **descobre um mundo já aberto** (via session-service) e **injeta** o jogador nele com o papel certo (`invader`/`coop`). Ver [contrato completo com sequence diagrams](../../docs/game-models/host_world_dropin-dataplane-contract.md).

- **Protocolo:** gRPC (unary + server-streaming) · **Portas host:** 50051 (gRPC), 8086 (HTTP: `/metrics`, `/healthz`)
- **Storage:** memória (fila e quadro de sinais com `sync.Mutex`; prod usaria Redis)
- **Dependências:** identity (JWT via interceptor gRPC) · **session-service** (gRPC — provisiona sessão/consulta mundos/injeta phantom) · **game-registry** (HTTP — perfil de multiplayer por jogo, ADR-0023)

## Contrato (protobuf — `proto/matchmaking/v1`)

```
service Matchmaking {
  rpc FindMatch(FindMatchRequest) returns (stream TicketUpdate);          // SEARCHING → MATCHED
  rpc Invade(InvadeRequest) returns (InvadeResponse);                     // unary: injeta na hora ou falha
  rpc PostSummonSign(PostSummonSignRequest) returns (stream SummonUpdate); // POSTED → SUMMONED | EXPIRED
  rpc ListSigns(ListSignsRequest) returns (ListSignsResponse);
  rpc SummonSign(SummonSignRequest) returns (SummonSignResponse);
}
```

Código gerado por **buf** (`buf generate`), commitado em `gen/`. Reflection habilitada (grpcurl sem o `.proto`).

## API

### Partida simétrica (`queue_arena`/etc)

| RPC | Tipo | Descrição |
|---|---|---|
| `FindMatch(game_id, mode_id)` | server-streaming | Enfileira o jogador e transmite `TicketUpdate` (`SEARCHING` → `MATCHED`/`FAILED`) até a partida formar. `MATCHED` traz `match_id`, `players`, `server_address` e o `connect_token` (claim `team_id`) do próprio jogador. Cancelar a chamada remove o ticket da fila. |

Ao formar o match, o matchmaking chama `CreateSession` no session-service (ver [Integrações](#integrações-externas)).

### Invasão e sinais (`host_world_dropin`)

Fonte de verdade do fluxo completo (com sequence diagrams): [`docs/game-models/host_world_dropin-dataplane-contract.md`](../../docs/game-models/host_world_dropin-dataplane-contract.md). Resumo dos RPCs:

| RPC | Tipo | Descrição |
|---|---|---|
| `Invade(game_id, soul_level, weapon_tier, mode_id, password)` | unary | O invasor descobre um mundo aberto elegível e é injetado nele na hora. Lista os mundos invadíveis do session (`ListInvadableWorlds`), filtra por faixa assimétrica e injeta (`JoinWorld(role=invader)`). Devolve `session_id` + `server_address` + `connect_token` (claim `role=invader`). `FailedPrecondition` (jogo não aceita invasão) / `NotFound` (nenhum mundo na faixa). O `player_id` vem do JWT, não do request. |
| `PostSummonSign(game_id, soul_level, weapon_tier, mode_id, password)` | server-streaming | O aliado anuncia um sinal e **aguarda** ser invocado: o stream entrega `POSTED{sign_id}` e depois `SUMMONED{session_id, server_address, connect_token(role=coop)}` (ou `EXPIRED` no TTL). Fechar o stream cancela o sinal. `FailedPrecondition` se o jogo não aceita co-op. |
| `ListSigns(game_id, host_soul_level, host_weapon_tier, mode_id, password)` | unary | O host vê os sinais compatíveis com a faixa do próprio mundo. |
| `SummonSign(session_id, sign_id)` | unary | O host escolhe um sinal; o aliado é injetado (`JoinWorld(role=coop)`) e recebe `server_address`/`connect_token` pelo stream aberto do `PostSummonSign`. `NotFound` (sinal inexistente) / `FailedPrecondition` (já consumido) / `ResourceExhausted` (sem vaga de co-op). |

O server do mundo (data plane Unity) não sabe nada sobre invasão/fila de sinais — só recebe conexões novas com o connect token e trata como join-in-progress. A capacidade por papel é garantida pelo `JoinWorld` do session-service (o matchmaking não reverifica cota).

## Integrações externas

- **session-service (gRPC, `SESSION_GRPC_ADDR`):** client único (`internal/sessionclient`) que satisfaz três Ports do domínio — `SessionProvisioner.Provision` (`CreateSession`, usado por `FindMatch`), `WorldDirectory.ListInvadableWorlds` (`ListInvadableWorlds`, usado por `Invade`) e `WorldInjector.Inject` (`JoinWorld`, usado por `Invade`/`SummonSign`). `codes.ResourceExhausted` do `CreateSession` vira `ErrNoServer`; do `JoinWorld` vira `ErrRoleFull`; `codes.NotFound` do `JoinWorld` vira `ErrWorldNotFound`.
- **game-registry (HTTP, `GAME_REGISTRY_URL` + `INTERNAL_API_KEY`):** client (`internal/registryclient`) usado para ler o perfil de multiplayer por jogo (ADR-0023) — `Format` (tamanho/composição do match, com cache de 30s via `profilecache`, caminho crítico do `FindMatch`) e a política de invasão/co-op (`InvasionPolicy`, sem cache — lida a cada `Invade`/`PostSummonSign`/`ListSigns`). Autentica com o header `X-Internal-Key` (ADR-0018), injetado pelo `http.Client` via `auth.InternalKeyTransport`.

## Modelo de dados

**Estado em memória** — sem persistência. Duas estruturas, ambas protegidas por `sync.Mutex`:

- **Fila de tickets** (`{ playerID, channel de Match }`): o worker forma matches FIFO e entrega cada `Match{ ID, Players, ConnectInfo }` pelo channel do ticket.
- **Quadro de sinais** (`host_world_dropin`, spec 007): cada sinal tem TTL (`signTTL`, 2 min) e um sweeper (`sweepInterval`, 15s) expira os vencidos, emitindo `EXPIRED` no stream do `PostSummonSign`.

Estado efêmero (some no restart); escala horizontal exigiria fila/quadro compartilhados (Redis).

## Env vars

| Var | Obrigatória | Default | Descrição |
|---|---|---|---|
| `SESSION_GRPC_ADDR` | sim (sem ela, provisionamento/injeção falham) | — | Endereço gRPC do session-service (`host:port`). |
| `GAME_REGISTRY_URL` | sim (fail-fast no boot) | — | Base URL HTTP do game-registry (perfil de multiplayer, ADR-0023). |
| `INTERNAL_API_KEY` | sim (sem ela, o game-registry responde 401) | — | Chave enviada em `X-Internal-Key` nas chamadas ao game-registry. |
| `IDENTITY_JWKS_URL` | sim | — | JWKS do identity para verificar o JWT dos RPCs (interceptor gRPC). |
| `GRPC_PORT` | não | `50051` | Porta do listener gRPC. |
| `PORT` | não | `8080` | Porta do HTTP auxiliar (`/healthz`, `/metrics`) — note que o compose expõe 8086 mapeado para essa porta. |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | não | vazio (tracing não exporta) | Endpoint OTLP gRPC (`host:port`) do collector. |
| `OTEL_TRACES_SAMPLER_RATIO` | não | `1.0` | Fração de traces amostrados. |

## Operação

- **Canal gRPC fica stale após recriar o session-service:** se o session-service for recriado/reiniciado (container novo, IP/porta reaproveitados), o `grpc.NewClient` deste serviço mantém uma conexão com referências antigas — chamadas a `CreateSession`/`ListInvadableWorlds`/`JoinWorld` passam a falhar ou pendurar mesmo com o session saudável. **Reinicie o matchmaking-service também** sempre que recriar o session-service (ex.: `docker restart <container-matchmaking>`).

## Observabilidade

Métricas custom em `/metrics` (porta 8086): `matchmaking_queue_size`, `matchmaking_matches_total`, `matchmaking_matches_failed_total` (fila/partida) e, para `host_world_dropin`, `invasions_total{result}` (`injected`/`no_world`/`not_supported`), `summons_total{result}` (`summoned`/`expired`/`cancelled`) e `summon_signs_active` — além das métricas runtime Go. Tracing via **otelgrpc**, propagado até o `CreateSession`/`JoinWorld` no session-service (ADR-0014). Aparece no **service graph** (edges `matchmaking → identity`, `matchmaking → session`, `matchmaking → game-registry`).

## Testar / Specs

- Manual (gRPC): [`api/README.md`](api/README.md) — exemplos com `grpcurl` (hoje cobre `FindMatch`; para `Invade`/`PostSummonSign`/`ListSigns`/`SummonSign` ver os `.http`/specs de cada fatia)
- Fluxo completo `host_world_dropin` (diagramas, connect token, eventos de participação): [`docs/game-models/host_world_dropin-dataplane-contract.md`](../../docs/game-models/host_world_dropin-dataplane-contract.md)
- Specs: [`specs/`](specs/) — `001-matchmaking-core`, `002-matchmaking-session-integration`, `003-game-scope`, `004-profile-driven-sizing`, `005-team-composition`, `006-world-discovery-invade`, `007-summon-signs`, `009-coop-mission-team`, `010-mode-scoped-matchmaking`, `011-password-matching`
