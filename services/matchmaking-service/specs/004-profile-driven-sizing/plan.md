# Plan: 004-profile-driven-sizing

**Serviço:** matchmaking-service · **Spec:** [spec.md](./spec.md) · **ADR:** 0023 (fatia 2)

## Objetivo

Substituir o `matchSize` global do matchmaker por um **tamanho derivado do perfil do jogo** (game-registry, fatia 1), com cache. Reusa padrões existentes: adapter HTTP com internal key (espelha `identity/internal/presence`), hexagonal (ADR-0002), fila concorrente (spec 001).

## Decisões técnicas

### D1 — Port `ProfileProvider` + adapter HTTP `registryclient`

- **Port (domínio):** `ProfileProvider.MatchSize(ctx, gameID) (int, error)`. O domínio só recebe o **tamanho** (int); a derivação ffa/teams mora no adapter.
- **Adapter `internal/registryclient`:** `GET {baseURL}/games/{id}/multiplayer-profile`, desserializa o perfil e **deriva**: `ffa`→`playerCount`; `teams`→`teamCount*teamSize`. Espelha `identity/internal/presence/client.go` (mesmo `http.Client` com `auth.InternalKeyTransport` + `otelhttp`).
- **Mapeamento de erro no adapter:** `404` (jogo/perfil ausente), arquétipo `!= queue_arena`, ou tamanho ≤ 0 → `domain.ErrProfileUnavailable`; erro de rede/status 5xx → também `ErrProfileUnavailable` (ver D4).

### D2 — Cache como **decorator** `ProfileProvider` (`profilecache`)

- `internal/profilecache`: um `CachingProvider` que **envolve** outro `ProfileProvider` e cacheia `gameID → (size, expiry)` em memória, sob `sync.Mutex`, com **TTL de 30s** (const no MVP; env `PROFILE_CACHE_TTL` como evolução).
- Cache-hit válido → devolve sem chamar o provider interno. Miss/expirado → chama, cacheia e devolve. **Erros não são cacheados** (para não fixar um jogo como "sem perfil" por 30s enquanto o registry está fora).
- Testável isoladamente: o caso "N chamadas dentro do TTL → 1 chamada" é teste **do decorator** (mock conta as chamadas), não do service.
- **Por quê decorator:** separa a preocupação de cache do adapter e do service; o matchmaker recebe um `ProfileProvider` e não sabe se é cacheado.

### D3 — Tamanho carimbado no ticket (remove o `matchSize` global)

- `ticket` ganha `matchSize int`. No `Join`, o matchmaker consulta `provider.MatchSize(ctx, gameID)`: erro → devolve `ErrProfileUnavailable` **antes** de enfileirar; sucesso → cria o ticket com o tamanho.
- `tryForm` agrupa por `gameID` (como hoje) e usa o **tamanho dos tickets do grupo** (`group[0].matchSize` — todos do mesmo jogo têm o mesmo) em vez de `m.matchSize`. O campo `m.matchSize` e o parâmetro do `New` **saem**.
- **Por quê no Join (não no tryForm):** falha rápida (o cliente sabe na hora que o jogo não tem perfil) e o tamanho fica pronto na formação, sem I/O dentro da seção crítica.

### D4 — Um erro só: `ErrProfileUnavailable` → gRPC `FailedPrecondition`

- No MVP, `ausente` / `arquétipo errado` / `registry fora` colapsam em `ErrProfileUnavailable`; o transport mapeia para `codes.FailedPrecondition`.
- **Trade-off honesto:** "registry fora" seria semanticamente `Unavailable` (retryável), distinto de "jogo sem perfil" (`FailedPrecondition`, permanente). Unificar é mais simples e o cliente trata igual ("não entrei na fila deste jogo"). A distinção fica como evolução (basta um segundo erro de domínio).

### D5 — Config: `GAME_REGISTRY_URL` obrigatório (fail-fast) + `INTERNAL_API_KEY`

- Sem o perfil o matchmaking **não forma nenhum match**; então `GAME_REGISTRY_URL` ausente → **erro no boot** (`run` retorna erro), em vez de aceitar tickets que nunca casam. `INTERNAL_API_KEY` alimenta o `auth.InternalKeyTransport` (ADR-0018).
- `docker-compose.yml` (serviço `matchmaking-service`) ganha as duas envs (`GAME_REGISTRY_URL: http://game-registry:8080`, `INTERNAL_API_KEY: dev-internal-key`).

## Modelo de dados

Nenhum. Fila e cache são em memória (spec 001). O perfil canônico vive no game-registry.

## Mudanças por camada (arquivos)

| Arquivo | Mudança |
|---|---|
| `internal/domain/matchmaking.go` | nova Port `ProfileProvider`; `Join` inalterado na assinatura |
| `internal/domain/errors.go` | `ErrProfileUnavailable` |
| `internal/registryclient/client.go` (novo) | adapter HTTP → game-registry; deriva o tamanho; satisfaz `ProfileProvider` |
| `internal/profilecache/cache.go` (novo) | `CachingProvider` decorator (TTL 30s, `sync.Mutex`) |
| `internal/service/matchmaker.go` | `New(interval, provider, ...)` (remove `matchSize`); `ticket.matchSize`; `Join` consulta o provider; `tryForm` usa o tamanho por grupo |
| `internal/transport/grpc_server.go` | mapear `ErrProfileUnavailable` → `codes.FailedPrecondition` |
| `cmd/server/main.go` | montar `profilecache.New(registryclient.New(url, httpClient), ttl)`; exigir `GAME_REGISTRY_URL`; `INTERNAL_API_KEY`; remover `matchSize` const |
| `infra/docker/docker-compose.yml` | `GAME_REGISTRY_URL` + `INTERNAL_API_KEY` no `matchmaking-service` |

## Testes (mapeiam os casos da spec)

- **service** (`matchmaker_test.go`, mock `ProfileProvider`): provider→2 forma com 2 tickets; provider→4 forma com 4; dois jogos com tamanhos diferentes (mock por `gameID`) formam matches independentes; provider→erro em `Join` → `ErrProfileUnavailable` e ticket **não** entra na fila. Rodar `-race`.
- **adapter** (`registryclient` + `httptest.Server`): derivação `ffa`→playerCount, `teams`→teamCount×teamSize; `404`→`ErrProfileUnavailable`; arquétipo ≠ queue_arena→`ErrProfileUnavailable`; injeta a internal key no header.
- **cache** (`profilecache`, provider mock que conta): N `MatchSize` do mesmo jogo dentro do TTL → 1 chamada; após expirar → rechama; erro não é cacheado.
- **transport** (gRPC): `Join`→`ErrProfileUnavailable` resulta em `FailedPrecondition`.

## Observabilidade

Métricas de fila/matches existentes seguem. Opcional (não bloqueia): contador de falhas de leitura de perfil. As chamadas HTTP ao game-registry entram no tracing via `otelhttp` no transport do client (como no identity). Sem exporter novo.

## Riscos / fora do escopo

- **Quebra de comportamento:** jogos **sem perfil** deixam de formar match (`FailedPrecondition`). O jogo-exemplo (arena) precisa de um perfil cadastrado (fatia 1). Documentar no smoke.
- Composição de times, resultado polimórfico, skill/MMR — fora (fatia 3 / backlog).
- Cache sem invalidação ativa — só TTL (janela de defasagem aceitável).
