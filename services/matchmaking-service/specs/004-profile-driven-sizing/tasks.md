# Tasks: 004-profile-driven-sizing

**Serviço:** matchmaking-service · **Spec:** [spec.md](./spec.md) · **Plan:** [plan.md](./plan.md)

Ordem hexagonal: domínio → adapters (registryclient, cache) → serviço → transporte → config → verificação. Marcar `[x]` conforme conclui.

## Domínio

- [x] **T1** — Em `internal/domain/matchmaking.go`: nova Port de saída `ProfileProvider` com `MatchSize(ctx context.Context, gameID string) (int, error)`. Comentário: devolve o tamanho derivado do perfil `queue_arena`; `ErrProfileUnavailable` quando indisponível.
- [x] **T2** — Em `internal/domain/errors.go`: `ErrProfileUnavailable` (jogo sem perfil `queue_arena` utilizável — ausente, arquétipo errado ou registry indisponível).

## Adapter game-registry (registryclient)

- [x] **T3** — `internal/registryclient/client.go` (novo): `Client` com `baseURL` + `*http.Client` (espelha `identity/internal/presence/client.go`). `New(baseURL, httpClient)`.
- [x] **T4** — `registryclient`: `MatchSize` faz `GET {baseURL}/games/{id}/multiplayer-profile`, desserializa o perfil e **deriva o tamanho**: `structure=="ffa"`→`playerCount`; `"teams"`→`teamCount*teamSize`. `404` / arquétipo ≠ `queue_arena` / tamanho ≤ 0 / status ≥ 400 / erro de rede → `domain.ErrProfileUnavailable` (erros wrappados). Satisfaz `domain.ProfileProvider`.

## Cache (decorator)

- [x] **T5** — `internal/profilecache/cache.go` (novo): `CachingProvider` que envolve um `domain.ProfileProvider`, cacheia `gameID → (size, expiry)` sob `sync.Mutex`, TTL const 30s. `New(inner, ttl)`. Satisfaz `domain.ProfileProvider`.
- [x] **T6** — `profilecache`: hit válido devolve sem chamar o inner; miss/expirado chama, cacheia e devolve; **erro não é cacheado** (rechama no próximo).

## Serviço (matchmaker)

- [x] **T7** — Em `internal/service/matchmaker.go`: `ticket` ganha `matchSize int`; `Matchmaker` remove o campo `matchSize` e ganha `provider domain.ProfileProvider`; `New(interval, provider, provisioner)` (remove o param `matchSize`).
- [x] **T8** — `matchmaker.go`: `Join` consulta `m.provider.MatchSize(ctx, gameID)` **antes** de enfileirar — erro → devolve `domain.ErrProfileUnavailable` (não enfileira); sucesso → cria o ticket com `matchSize`.
- [x] **T9** — `matchmaker.go`: `tryForm` usa o tamanho **por grupo** (`group[0].matchSize`) no lugar de `m.matchSize` ao fatiar os tickets de cada jogo.

## Transporte (gRPC)

- [x] **T10** — Em `internal/transport/grpc_server.go`: mapear `domain.ErrProfileUnavailable` → `status.Error(codes.FailedPrecondition, ...)` no caminho do `FindMatch`/`Join`.

## Config / wire-up

- [x] **T11** — `cmd/server/main.go`: ler `GAME_REGISTRY_URL` (ausente → `run` retorna erro, fail-fast) e `INTERNAL_API_KEY`; montar `httpClient` com `auth.InternalKeyTransport(internalKey, otelhttp.NewTransport(...))`; `provider := profilecache.New(registryclient.New(url, httpClient), 30*time.Second)`; `service.New(formInterval, provider, provisioner)`; remover a const `matchSize`.
- [x] **T12** — `infra/docker/docker-compose.yml` (serviço `matchmaking-service`): `GAME_REGISTRY_URL: http://game-registry:8080` e `INTERNAL_API_KEY: dev-internal-key`; `depends_on` game-registry (se coerente com o compose).

## Testes (mapeiam os casos da spec)

- [x] **T13** — `internal/service/matchmaker_test.go` (mock `ProfileProvider`): provider→2 forma com 2 tickets; provider→4 forma com 4; dois jogos com tamanhos diferentes (mock por `gameID`) formam matches independentes; provider→erro em `Join` → `ErrProfileUnavailable` e fila não cresce. Ajustar os testes existentes ao novo `New` (sem `matchSize`). `-race`.
- [x] **T14** — `internal/registryclient/client_test.go` (novo, `httptest.Server`): derivação `ffa`→playerCount, `teams`→teamCount×teamSize; `404`→`ErrProfileUnavailable`; arquétipo ≠ queue_arena→`ErrProfileUnavailable`; header da internal key presente.
- [x] **T15** — `internal/profilecache/cache_test.go` (novo, mock que conta chamadas): N `MatchSize` dentro do TTL → 1 chamada ao inner; após expirar → rechama; erro não cacheado (2 chamadas em 2 erros).
- [x] **T16** — `internal/transport/grpc_server_test.go`: `Join`→`ErrProfileUnavailable` resulta em `codes.FailedPrecondition`.

## Verificação final

- [x] **T17** — `go build ./services/matchmaking-service/...`, `go test ./services/matchmaking-service/... -race -cover` (≥80% em service), `golangci-lint`, `gofmt`.
- [x] **T18 (smoke)** — subir game-registry + matchmaking; cadastrar um **perfil** no jogo-exemplo (PUT `/games/{id}/multiplayer-profile`, `ffa playerCount=2`); dois `FindMatch` do jogo → formam match no tamanho do perfil; jogo **sem** perfil → `FailedPrecondition`.
- [x] **T19** — Aprendizado (CLAUDE.md): se surgir algo novo de Go (ex.: **decorator de uma Port** para cache, ou `http.RoundTripper` de auth já coberto em #16), registrar em `docs/GO-LEARNINGS.md`; senão, dizer que não houve novidade.
