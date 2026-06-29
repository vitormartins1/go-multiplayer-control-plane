# Plan: 004-games
**Spec:** specs/004-games/spec.md
**Data:** 2026-06-29

## Abordagem técnica

Adiciona um adapter `gameregistry` (Port `GameRegistry`) e estende o adapter `achievements` para ser game-scoped, no mesmo padrão dos demais (raw HTTP + otelhttp). Novidade: como há **writes** (criar jogo/conquista), os erros de validação do upstream (4xx) são repassados via um `UpstreamError{Status}`; indisponibilidade (conn/5xx) vira `Err...Unavailable` -> 502. Um helper único classifica a resposta: 2xx ok; 4xx -> `UpstreamError`; conn/5xx -> indisponível.

## Arquivos a criar ou modificar

- `internal/domain/admin.go` — tipos `Game{ID,Slug,Name,CreatedAt}`, `Achievement{ID,Name,Description,Target}` (catálogo); Port `GameRegistry` (`ListGames`, `CreateGame`); estende `AchievementsReader` (`CatalogForGame`, `CreateAchievement`, `PlayerAchievements`; remove `ForPlayer`)
- `internal/domain/errors.go` — `ErrGameRegistryUnavailable`; tipo `UpstreamError{Status int}` (implementa `error`)
- `internal/gameregistry/client.go` (+ test) — `ListGames` (`GET /games`), `CreateGame` (`POST /games`); classifica resposta (4xx -> UpstreamError; conn/5xx -> ErrGameRegistryUnavailable)
- `internal/achievements/client.go` (+ test) — trocar `ForPlayer` pelas rotas game-scoped: `CatalogForGame` (`GET /admin/games/{g}/achievements`), `CreateAchievement` (`POST ...`), `PlayerAchievements` (`GET /admin/games/{g}/players/{p}/achievements`); mesmo tratamento de erro
- `internal/service/admin_service.go` — `New` recebe o `GameRegistry`; métodos `Games`, `CreateGame`, `GameCatalog`, `CreateAchievement`, `PlayerAchievements(gameID, playerID)` (remove o `PlayerAchievements(playerID)` antigo)
- `internal/service/admin_service_test.go` — atualizar `New(...)`, fakes (game registry + achievements game-scoped), novos testes
- `internal/transport/http_handler.go` — rotas novas (`/api/games`, `/api/games/{gameId}/achievements`, `/api/games/{gameId}/players/{playerID}/achievements`); remove `GET /api/players/{playerID}/achievements`; helper de erro repassa `UpstreamError.Status` e mapeia indisponível -> 502
- `internal/transport/http_handler_test.go` — atualizar `fakeService`; testes das rotas novas (200/201/400/409/502)
- `cmd/server/main.go` — lê `GAME_REGISTRY_URL`; cria o `gameregistry` client (otelhttp); injeta no `service.New`
- `api/README.md` + `README.md` — documentar as rotas de jogos/achievements por jogo
- `infra/docker/docker-compose.yml` — `admin-bff`: `GAME_REGISTRY_URL: http://game-registry:8080` + `depends_on: game-registry`

## Decisões técnicas e trade-offs
- **`UpstreamError{Status}` para repassar 4xx**: num proxy de escrita, "slug já existe" (409) e "slug inválido" (400) têm que chegar ao painel — não podem virar 502. Indisponibilidade (conn/5xx) continua 502. Um helper único classifica para os dois adapters.
- **Substituir `ForPlayer` por `PlayerAchievements(gameID, playerID)`**: o achievements ficou game-scoped (004), então o adapter acompanha; a rota antiga `/api/players/{id}/achievements` sai.
- **Adapter próprio por dependência** (gameregistry, achievements, identity, prometheus): cada um fala seu contrato; consistente com o que já existe.
- **`GAME_REGISTRY_URL` no `main`** (não em shared/config): igual às outras envs do BFF.

## Ordem de implementação
1. `domain` (tipos Game/Achievement; Port GameRegistry; estende AchievementsReader; `UpstreamError`/erro)
2. `gameregistry` client + test (`httptest`: list/create; 4xx -> UpstreamError; fora -> unavailable)
3. `achievements` client: rotas game-scoped + test
4. `service` (Games/CreateGame/GameCatalog/CreateAchievement/PlayerAchievements) + tests
5. `transport` (rotas + repasse de UpstreamError + 502) + tests
6. wire-up `main` (GAME_REGISTRY_URL) + compose + READMEs
7. `go test ./... -race`; `go build ./...`; validar no Docker (listar/criar jogo, cadastrar/listar achievements por jogo, repasse de 409)

## Riscos e pontos de atenção
- **Shape de erro do upstream**: o body de erro do game-registry/achievements é `{"error":"slug_taken"}` etc.; o BFF repassa o **status**; repassar o código de erro do body é opcional (a UI pode mapear pelo status). Decidir se inclui o code no corpo.
- **Quebra resolvida**: esta feature restaura a busca de achievements do painel (game-scoped); validar que o `admin-ui` antigo (que chama `/api/players/{id}/achievements`) será atualizado no `admin-ui/003` (próxima) — até lá o painel de achievements fica sem essa rota.
- **otelhttp** no client do game-registry (trace cross-service).
- **Concorrência**: nada novo; service permanece sem estado.
