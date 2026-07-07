# Plan: 006-world-discovery-invade

**Serviço:** matchmaking-service · **Spec:** `spec.md` (Aprovada 2026-07-07) · **ADR:** 0023 (H1a, lado matchmaking)
**Depende de:** `session/012` (ListInvadableWorlds + atributos) e `session/011` (JoinWorld) — **implementar `session/012` primeiro** (os tipos gerados do session são consumidos aqui).

Como o matchmaking ganha o **segundo modo** (descobrir + injetar). O queue-form (`FindMatch`, specs 004/005) não regride — é caminho **totalmente separado**.

## Decisões técnicas

### D1 — `Invade` unary + `UnaryAuthInterceptor` (o de hoje é stream-only)

O interceptor de auth atual (`StreamAuthInterceptor`) só cobre streams; `FindMatch` é o único RPC. `Invade` é **unary**, então:

- Novo `UnaryAuthInterceptor(verifier)` espelhando o stream: `bearerFromMetadata` → `verifier.Verify` → injeta `playerID` no context (reusa `playerIDKey`/`playerIDFromContext`, já no pacote). Registrado no `main` via `grpc.ChainUnaryInterceptor`.
- O `playerID` do invasor vem do **JWT** (context), não do request — igual ao `FindMatch`.
- `transport.Server` passa a segurar também o serviço de invasão: `NewServer(mm domain.Matchmaker, inv domain.Invader)`.

### D2 — leitura da política de invasão: Port nova `InvasionPolicyProvider` (não mexe no cache do queue-form)

O `ProfileProvider.Format` (queue_arena) devolve `MatchFormat` e é embrulhado pelo `profilecache`. Invasão precisa de outra informação (`archetype` + flags). Para **não** tocar o `Format`/cache do caminho crítico:

- Nova Port `InvasionPolicyProvider`: `InvasionPolicy(ctx, gameID) (InvasionPolicy, error)` com `InvasionPolicy{Archetype string; InvasionEnabled, LevelRangeEnabled, WeaponTierEnabled bool}`.
- Implementada pelo **mesmo** `registryclient.Client` (um struct pode satisfazer as duas Ports): novo método que bate no mesmo `GET /games/{id}/multiplayer-profile` e lê `archetype` + `params.{invasionEnabled,levelRangeEnabled,weaponTierEnabled}`. Arquétipo ≠ `host_world_dropin` → devolve a policy com `Archetype` preenchido (o service decide `ErrInvasionNotSupported`); rede/404 → `ErrProfileUnavailable`.
- **Sem cache** nesta fatia (invasão não tem a frequência do `Format` por ticket). Se virar quente, embrulha depois.

### D3 — `WorldDirectory` + `WorldInjector`: métodos novos no `sessionclient.Client`

O `sessionclient.Client` já fala o `sessionv1.SessionClient` (implementa `SessionProvisioner`). Adiciono duas Ports que o mesmo struct satisfaz, reusando o client gRPC:

- `WorldDirectory.ListInvadableWorlds(ctx, gameID) ([]domain.InvadableWorld, error)` → chama `session.ListInvadableWorlds`, traduz `[]*sessionv1.InvadableWorld` → `[]domain.InvadableWorld`.
- `WorldInjector.Inject(ctx, sessionID, playerID, role) (domain.Injected, error)` → chama `session.JoinWorld`, traduz a resposta (`grant.connect_token` + precisa do `server_address`).

> **Atenção (contrato H0):** o `JoinWorldResponse` da H0 devolve **só** o `ConnectGrant` (token), **não** o `server_address`. Mas o invasor precisa do endereço do server para conectar. Duas saídas: **(a)** `session/012` adiciona `server_address` ao `JoinWorldResponse` (aditivo, trivial); **(b)** o matchmaking já tem o `sessionID` e faz outra chamada. **Recomendação: (a)** — incluir `server_address` no `JoinWorldResponse` é a forma certa (o phantom sempre precisa dele). **Isso é um ajuste em `session/012`** (ou um adendo à H0): anotar lá antes de implementar. Sem ele, `Invade` não tem o que devolver no `server_address`.

### D4 — serviço de aplicação `Invader` (pacote `service`)

Novo `internal/service/invader.go`, Port de entrada `domain.Invader`:

```go
Invade(ctx, gameID, playerID string, crit domain.InvaderCriteria) (domain.Invaded, error)
```

Orquestração:
1. `policy, err := policies.InvasionPolicy(ctx, gameID)` — erro → propaga (`ErrProfileUnavailable`).
2. `policy.Archetype != "host_world_dropin" || !policy.InvasionEnabled` → `ErrInvasionNotSupported`.
3. `worlds, _ := directory.ListInvadableWorlds(ctx, gameID)`.
4. filtra elegíveis (D5), **na ordem retornada**.
5. para cada elegível: `injector.Inject(ctx, w.SessionID, playerID, "invader")`; sucesso → devolve `Invaded{ServerAddress, ConnectToken, SessionID}`; `ErrRoleFull` (corrida) → tenta o próximo; outro erro → propaga.
6. esgotados/vazio → `ErrNoEligibleWorld`.
- As chamadas ao session usam `context.WithTimeout` (reusar `provisionTimeout` = 5s ou um `invadeTimeout` próprio).
- Contadores: `map[string]int64` (`injected`/`no_world`/`not_supported`) + `InvasionsTotal(result) float64` (métrica; padrão do `sessions_closed_total`).

### D5 — elegibilidade por faixa: função de domínio pura

`internal/domain` ganha `InvaderCriteria{SoulLevel, WeaponTier int}` e uma função pura testável:

```go
// eligible decide se um host é invadível pelo invasor, conforme a política.
func Eligible(crit InvaderCriteria, w InvadableWorld, p InvasionPolicy) bool
```

- `LevelRangeEnabled=false` → ignora nível.
- `LevelRangeEnabled=true` → banda **assimétrica** (invasão "punge para cima"): host `H` elegível para invasor `L` sse `H ∈ [L - below(L), L + above(L)]`, com `above > below` (ex.: `below = 0.1·L`, `above = 0.1·L + 20` → invasor L=100 casa hosts [90,130]). **Constantes nomeadas e tunáveis** no domínio; números ilustrativos da ficha §4, não os exatos da FromSoftware.
- `WeaponTierEnabled=true` → exige `|hostTier - invaderTier| <= tierBand` (ex.: 1); `false` → ignora.
- Mundo sem atributos (host soul level 0) com `LevelRangeEnabled=true` → **não** elegível (não dá para casar por faixa) — decisão explícita, testada.

### D6 — proto + transport + métricas

- Proto: `Invade(InvadeRequest{game_id, int32 soul_level, int32 weapon_tier}) → InvadeResponse{session_id, server_address, connect_token}`. `buf generate`.
- Transport handler `Invade`: `playerIDFromContext`, valida `game_id`, chama `inv.Invade`, mapeia erros → codes: `ErrInvasionNotSupported`→`FailedPrecondition`, `ErrNoEligibleWorld`→`NotFound`, `ErrProfileUnavailable`→`FailedPrecondition`, default→`Internal`.
- Métrica: 3× `CounterFunc` `invasions_total{result}` no `main` (const labels, padrão do session).

## Arquivos a tocar

| Arquivo | Mudança |
|---|---|
| `proto/matchmaking/v1/matchmaking.proto` | RPC `Invade` + messages; `buf generate` |
| `internal/domain/matchmaking.go` | `InvaderCriteria`, `InvadableWorld`, `Injected`, `Invaded`, `InvasionPolicy`; Ports `Invader`, `InvasionPolicyProvider`, `WorldDirectory`, `WorldInjector`; `Eligible` |
| `internal/domain/errors.go` | `ErrInvasionNotSupported`, `ErrNoEligibleWorld` (reusa `ErrProfileUnavailable`) |
| `internal/service/invader.go` (novo) | serviço `Invader` + orquestração + contadores |
| `internal/service/invader_test.go` (novo) | ramos do Invade + `Eligible` (tabela) |
| `internal/registryclient/client.go` | método `InvasionPolicy` (+ teste) |
| `internal/sessionclient/grpc_client.go` | `ListInvadableWorlds` + `Inject` (+ teste) |
| `internal/transport/grpc_server.go` | handler `Invade` + `UnaryAuthInterceptor` (+ teste) |
| `cmd/server/main.go` | wire do `Invader` (policy=registryclient cru, directory/injector=sessionclient), unary interceptor, métricas |

## Testes

- **Baseline** `go test ./services/matchmaking-service/... -race` verde antes.
- `domain`: `Eligible` em tabela — dentro/fora da banda (assimetria: L=100 casa 90 e 130, não 89 nem 131), `LevelRangeEnabled=false` casa qualquer, weapon tier on/off, host sem atributos.
- `service` (mocks das 3 Ports): `Invade` feliz (injeta 1º elegível); arquétipo errado / invasão off → `ErrInvasionNotSupported`; lista vazia / nada na faixa → `ErrNoEligibleWorld`; 1º elegível cheio (`ErrRoleFull`) → tenta 2º e injeta; todos cheios → `ErrNoEligibleWorld`; contadores certos.
- `registryclient`: parse da policy (invasionEnabled etc.); 404 → `ErrProfileUnavailable` (httptest).
- `sessionclient`: `Inject` mapeia `ResourceExhausted`→`ErrRoleFull`; `ListInvadableWorlds` traduz. (bufconn com um stub do SessionServer, ou mock do `sessionv1.SessionClient`.)
- `transport`: mapa de erros do `Invade` → codes; `UnaryAuthInterceptor` (token ausente → `Unauthenticated`).
- Cobertura ≥80% em `service/`/`domain/`.

## Sequência de tasks (preview)

1. Baseline verde. **Pré-requisito:** `session/012` implementado e mergeado (inclui `server_address` no `JoinWorldResponse` — D3).
2. Domínio: tipos + Ports + erros + `Eligible` (+ teste tabela).
3. `registryclient.InvasionPolicy` (+ teste).
4. `sessionclient`: `ListInvadableWorlds` + `Inject` (+ teste).
5. Serviço `Invader` + contadores (+ testes com mocks).
6. Proto `Invade` + `buf` + transport (handler + `UnaryAuthInterceptor`) + testes.
7. Wire-up `main` (invader + unary interceptor + métricas).
8. `go test ./... -race` verde; lint; validação e2e (grpcurl ou teste bufconn cruzando os dois serviços — opcional).
9. GO-LEARNINGS se surgir algo (candidato: "segundo modo" sem fila — orquestração stateless list→filter→inject com retry; ou um struct satisfazendo múltiplas Ports).

## Riscos / gotchas

- **`JoinWorldResponse` sem `server_address`** (D3) — bloqueia o retorno do `Invade`. Resolver em `session/012` **antes** (ajuste aditivo no proto do session). É a dependência mais fácil de esquecer.
- **Interceptor unary** — o de hoje é stream-only; sem o unary, `Invade` roda sem `playerID` (viria vazio). Não esquecer o `ChainUnaryInterceptor` no `main`.
- **`buf generate`** nos dois serviços (session e matchmaking), nunca `sed`.
- **Corrida listar→injetar** é esperada e tratada (retry no próximo); não tentar reservar slot no diretório.
- Perfil host_world sem cache — ok para a frequência da invasão; não replicar o `profilecache` sem necessidade.
