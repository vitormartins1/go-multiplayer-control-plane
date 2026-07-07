# Plan: 007-summon-signs

**Serviço:** matchmaking-service · **Spec:** `spec.md` (Aprovada 2026-07-07) · **ADR:** 0023 (H1b, co-op por sinais)
**Depende de:** `session/011` (`JoinWorld` role=coop) e `matchmaking/006` (reusa `WorldInjector`, o registryclient e o padrão de elegibilidade pura).

Como o matchmaking ganha o **terceiro modo** (quadro de sinais de co-op, dois passos). O queue-form (`FindMatch`) e a invasão (`Invade`) **não regridem** — é caminho independente.

## Decisões técnicas

### D1 — o store dos sinais é **estado interno do serviço**, não uma Port

A spec cogitou uma Port `SummonBoard`. **Refino aqui:** o quadro é estado efêmero em memória com **canal de entrega por sinal** — a mesma natureza da **fila do `FindMatch`**, que o `Matchmaker` guarda como `queue []*ticket` **interno** (não atrás de Port). Seguir o vizinho: o `Summoner` (novo serviço) guarda `signs map[string]*sign` + `sync.Mutex` + `now func() time.Time`, e o `sign` interno embrulha o `domain.Sign` com o canal e o estado. Menos Ports, consistente com o `Matchmaker`. Motivo reforçado: o canal Go de entrega **não** atravessa Redis (multi-instância = pub/sub, evolução declarada).

### D2 — `PostSummonSign` server-streaming, espelhando o `FindMatch`

O aliado anuncia e **aguarda**. O handler:
1. `svc.PostSign(...)` → `(signID, <-chan domain.Summoned, err)`; erro `ErrCoopNotSupported` → `FailedPrecondition`.
2. envia `POSTED{sign_id}`.
3. `select`:
   - `<-stream.Context().Done()` (aliado fechou) → `svc.CancelSign(signID)` e retorna `ctx.Err()`.
   - `s, ok := <-ch`: `ok` → `SUMMONED{session_id, server_address, connect_token}`; `!ok` (canal fechado pelo sweeper de TTL) → `EXPIRED`.

O canal tem **buffer 1** (a entrega nunca bloqueia o `Summon`, igual ao `ticket.ch`). Entrega = `send`; expiração/cancelamento = `close` sem valor (o handler distingue por `ok`).

### D3 — ciclo de vida do sinal: `active → claimed → (consumido | active)`

A invocação precisa ser **atômica** e **reversível se o mundo estiver cheio** (a spec exige: role full → sinal **não** consumido). Estado do `sign`:

- **Post** → `active`, registra o canal.
- **List(gameID)** → só `active`.
- **Summon(sessionID, signID)**:
  1. sob o lock: acha o sinal; ausente → `ErrSignNotFound`; já `claimed`/consumido → `ErrSignConsumed`; senão marca `claimed` (transição atômica — só um host ganha).
  2. fora do lock: `injector.Inject(sessionID, allyID, "coop")`.
  3. sucesso → entrega `Summoned` no canal, **remove** o sinal (consumido), conta `summoned`, devolve `allyID`.
  4. `ErrRoleFull`/`ErrWorldNotFound` → **volta** o sinal para `active` (release) e devolve o erro mapeado (sinal segue disponível).
- **CancelSign(signID)** (aliado fechou o stream): se existe e não está `claimed` → remove + `close(ch)`, conta `cancelled`. Idempotente.
- **SweepExpired(ttl)**: sinais `active` com `postedAt` antes de `now-ttl` → remove + `close(ch)` (o handler manda `EXPIRED`), conta `expired`.

`claimed` fora da List garante que dois hosts não invoquem o mesmo sinal em paralelo.

### D4 — política de co-op: Port irmã `CoopPolicyProvider` no registryclient

A H1a leu `InvasionPolicy{Archetype, InvasionEnabled, LevelRangeEnabled, WeaponTierEnabled}`. O co-op precisa saber se o papel **`coop`** está habilitado — que vem de `params.enabledRoles` (lista), não de um `coopEnabled` isolado. Para **não** tocar a `InvasionPolicy` da H1a:

- Nova Port `CoopPolicyProvider`: `CoopPolicy(ctx, gameID) (CoopPolicy, error)` com `CoopPolicy{Archetype string; CoopEnabled, LevelRangeEnabled, WeaponTierEnabled bool}` e `SupportsCoop()` (`Archetype==host_world_dropin && CoopEnabled`).
- Implementada pelo **mesmo** `registryclient.Client` (mais uma Port no mesmo struct — padrão da H1a): estende `profileResponse.Params` com `EnabledRoles []string` (`json:"enabledRoles"`); `CoopEnabled` = contém `"coop"`. Reusa o helper `getProfile`. 404/rede → `ErrProfileUnavailable`. **Sem cache** (como a invasão).

### D5 — elegibilidade de co-op: banda **simétrica** (função pura)

`domain` ganha `CoopCriteria{SoulLevel, WeaponTier int}` e a função irmã de `Eligible`:

```go
func CoopEligible(ally CoopCriteria, hostSoulLevel, hostWeaponTier int, p CoopPolicy) bool
```

- `LevelRangeEnabled=false` → ignora nível.
- `LevelRangeEnabled=true` → banda **simétrica** em torno do nível do **aliado** `A` (ficha §4: co-op `X ± (10 + 10%)`): host `H` casa sse `H ∈ [A - (coopFlat + coopFraction·A), A + (coopFlat + coopFraction·A)]`, com `coopFlat=10`, `coopFraction=0.10`. Diferente da invasão (assimétrica). Sinal/host sem nível (0) com range on → **não** casa.
- `WeaponTierEnabled=true` → `|hostWeaponTier - ally.WeaponTier| <= weaponTierBand` (reusa a const da H1a). `false` → ignora.

Constantes nomeadas e tunáveis, ao lado das da invasão em `invasion.go` (ou um `coop.go` no domínio — decisão de arquivo, mesma pasta).

### D6 — serviço de aplicação `Summoner` + contadores

`internal/service/summoner.go`, Port de entrada `domain.Summoner`:

```go
PostSign(ctx, gameID, playerID string, crit CoopCriteria) (signID string, updates <-chan Summoned, err error)
ListSigns(ctx, gameID string, host CoopCriteria) ([]Sign, error)
Summon(ctx, sessionID, signID string) (allyID string, err error)
CancelSign(signID string)
```

- `PostSign`/`ListSigns` leem `CoopPolicy` → `SupportsCoop()` senão `ErrCoopNotSupported`. `ListSigns` filtra `active` por `CoopEligible`.
- Contadores: `map[string]int64` (`summoned`/`expired`/`cancelled`) + `SummonsTotal(result) float64`. Gauge de ativos: `SignsActive() float64` (total; label por jogo fica p/ H3 — cardinalidade).
- `RunSweeper(ctx, ttl, interval)` — goroutine no main (padrão do session `RunSweeper`). `signTTL`/`sweepInterval` como constantes.
- Timeout nas leituras/injeção ao mundo (reusa o padrão `context.WithTimeout` da invasão).

### D7 — proto + transport + métricas

- Proto: `PostSummonSign` (stream), `ListSigns` (unary), `SummonSign` (unary) + messages + `enum SummonStatus`. `buf generate`.
- `transport.Server` passa a segurar as **três** Ports de entrada: `NewServer(mm domain.Matchmaker, inv domain.Invader, sum domain.Summoner)`.
- Handlers: `PostSummonSign` (stream, D2 — usa o `StreamAuthInterceptor` já existente); `ListSigns`/`SummonSign` (unary — usam o `UnaryAuthInterceptor` da H1a). Mapa de erros: `ErrCoopNotSupported`→`FailedPrecondition`, `ErrSignNotFound`/`ErrWorldNotFound`→`NotFound`, `ErrSignConsumed`→`FailedPrecondition`, `ErrRoleFull`→`ResourceExhausted`.
- Métricas no main: `summons_total{result}` (3× CounterFunc com ConstLabels, padrão da H1a `invasions_total`) + `summon_signs_active` gauge.

## Arquivos a tocar

| Arquivo | Mudança |
|---|---|
| `internal/domain/coop.go` (novo) | `Sign`, `Summoned`, `CoopCriteria`, `CoopPolicy`(+`SupportsCoop`), `CoopEligible`+consts; Ports `Summoner`, `CoopPolicyProvider` |
| `internal/domain/coop_test.go` (novo) | `CoopEligible` (tabela) |
| `internal/domain/errors.go` | `ErrCoopNotSupported`, `ErrSignNotFound`, `ErrSignConsumed` |
| `internal/registryclient/client.go` | `CoopPolicy` (+ `enabledRoles` no parse) |
| `internal/registryclient/client_test.go` | testes de `CoopPolicy` |
| `internal/service/summoner.go` (novo) | serviço `Summoner` + store interno + sweeper + contadores |
| `internal/service/summoner_test.go` (novo) | ramos (post/list/summon/cancel/expira/corrida) com mocks |
| `proto/matchmaking/v1/matchmaking.proto` | 3 RPCs + messages + enum; `buf generate` |
| `internal/transport/grpc_server.go` | 3 handlers + `NewServer(mm, inv, sum)` |
| `internal/transport/grpc_server_test.go` | e2e dos 3 RPCs via bufconn |
| `cmd/server/main.go` | wire do `Summoner` (coop policy = registryclient; injector = sessionclient), sweeper, métricas, `NewServer(mm, inv, sum)` |

## Testes

- **Baseline** `go test ./services/matchmaking-service/... -race` verde antes.
- `domain`: `CoopEligible` em tabela — banda **simétrica** (A=100 casa 80 e 120, não 79 nem 121), `LevelRangeEnabled=false` casa qualquer, weapon tier on/off, nível 0 com range on → não casa.
- `service` (mocks de policy + injector): `PostSign` feliz (publica + canal); não suportado → `ErrCoopNotSupported`; `ListSigns` filtra por faixa; `Summon` feliz (claim→inject→entrega no canal→consome); sinal inexistente → `ErrSignNotFound`; já consumido → `ErrSignConsumed`; corrida (dois `Summon` no mesmo sinal — só um injeta); `ErrRoleFull` → sinal segue `active` (não consumido); `CancelSign` remove; `SweepExpired` fecha o canal e conta.
- `registryclient`: parse de `enabledRoles` → `CoopEnabled`; arquétipo errado → `!SupportsCoop`; 404 → `ErrProfileUnavailable`.
- `transport` (bufconn): `PostSummonSign` recebe `POSTED`; `SummonSign` entrega `SUMMONED` no stream do aliado; `ListSigns` reflete; erros → codes. Token ausente → `Unauthenticated` (interceptors já cobertos).
- Cobertura ≥80% em `service/`/`domain/`.

## Sequência de tasks (preview)

1. Baseline verde.
2. Domínio: tipos + Ports + erros + `CoopEligible` (+ teste tabela).
3. `registryclient.CoopPolicy` (+ teste).
4. Serviço `Summoner` + store + sweeper + contadores (+ testes com mocks).
5. Proto (3 RPCs + enum) + `buf` + transport (3 handlers, `NewServer(mm,inv,sum)`) + testes bufconn.
6. Wire-up `main` (summoner + sweeper + métricas).
7. `go test ./... -race` verde; lint; `FindMatch`/`Invade` sem regressão.
8. GO-LEARNINGS se surgir algo (candidato: entrega assíncrona por canal fechado vs. enviado — `ok` do receive como discriminador SUMMONED/EXPIRED; claim/release reversível). Atualizar a tabela de estado do README de game-models (H1b).

## Riscos / gotchas

- **Entrega por canal + fechar vs. enviar:** o handler distingue `SUMMONED` (valor) de `EXPIRED` (canal fechado) pelo `ok` do receive. Fechar um canal já drenado é seguro; **enviar** em canal fechado dá panic — por isso `Summon` (envia) e `SweepExpired`/`CancelSign` (fecham+removem) são mutuamente exclusivos sob o lock, e um sinal `claimed` **não** é varrido/cancelado.
- **Corrida claim:** a transição `active→claimed` sob o lock é o que garante "um sinal, um host". O `Inject` roda **fora** do lock (rede) — release volta a `active` só em falha de capacidade.
- **`buf generate`** nos dois? Só matchmaking (o session não muda nesta fatia). Nunca `sed`.
- **Store em memória** — sinais somem no restart; ok (presença efêmera). Redis/pub-sub = evolução.
