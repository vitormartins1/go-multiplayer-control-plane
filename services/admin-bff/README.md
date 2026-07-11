# admin-bff

Back-end-for-frontend do **painel de administração** ("sala de controle" do backend de jogos — ADR-0016). Agregador **read-only e stateless**: não tem banco próprio, não decide regra de negócio — cada handler chama um ou mais serviços upstream, traduz o resultado (e o erro) e devolve JSON pro `admin-ui` (SPA React, repo/pasta à parte). Responsabilidade distinta do Grafana: produto/admin, não séries/infra.

- **Protocolo:** REST (`net/http`, `ServeMux` do Go 1.22+ com path params) · **Porta host:** 8088 (API, `/metrics`, `/healthz`)
- **Storage:** nenhum — puro agregador
- **Upstreams (9, cada um com client próprio em `internal/<nome>/client.go` + Port em `internal/domain/`):**

  | Client (`internal/`) | Serve | Env var |
  |---|---|---|
  | `prometheus` | topologia, contadores ao vivo | `PROMETHEUS_URL` |
  | `achievements` | catálogo, progresso, import CSV, event rules | `ACHIEVEMENTS_URL` |
  | `identity` | User base (players) | `IDENTITY_URL` |
  | `gameregistry` | jogos, fleet, perfis/modos multiplayer | `GAME_REGISTRY_URL` |
  | `leaderboard` | placares por jogo | `LEADERBOARD_URL` |
  | `playerdata` | dados arbitrários do player | `PLAYER_DATA_URL` |
  | `cloudsave` | metadados de save do player | `CLOUD_SAVE_URL` |
  | `session` | sessões/rooms (board global e por jogo) | `SESSION_URL` |
  | `analytics` | overview e analytics por jogo | `ANALYTICS_URL` |

  URL vazia não impede o boot — só loga warning e degrada aquele recurso em runtime (ver `cmd/server/main.go`).
- **Auth de entrada:** nenhuma no MVP (interno/local; auth real do `admin-ui` é evolução — provável ADR futuro).
- **Auth de saída:** todo client injeta `X-Internal-Key` (`INTERNAL_API_KEY`, ADR-0018) via `auth.InternalKeyTransport` no `http.Client` compartilhado — é assim que o admin-bff acessa os endpoints `/admin/...` dos outros serviços. Não confundir com auth de entrada: quem chama o admin-bff não precisa de token.

## Padrão de agregação

Todo handler segue a mesma forma: 1) extrai path/query params, 2) chama o `Service` (que chama um ou mais clients upstream via Port em `internal/domain/`), 3) em erro, traduz via `writeUpstream` (ou, nos handlers mais antigos ainda não migrados, `errors.Is` manual contra os sentinels de `internal/domain/`), 4) em sucesso, serializa a resposta.

**O padrão que importa pra quem for adicionar um endpoint é `UpstreamError`** (`internal/domain/errors.go`): o client de saída não decide o status HTTP que o admin-bff devolve — ele classifica a resposta upstream e propaga. Isso vive em `internal/upstream/upstream.go` (`upstream.Do`): 2xx passa a resposta pro caller; 4xx fecha o body e devolve `*domain.UpstreamError{Status, Code}` com o status **original** do upstream (ex.: 409 de slug duplicado no game-registry chega como 409 no admin-ui, não mascarado); 5xx/erro de conexão devolve o sentinel `ErrXUnavailable` daquele serviço, wrappado.

No handler, `writeUpstream` faz o `errors.As`/`errors.Is` e escreve a resposta:

```go
// client (internal/gameregistry/client.go): delega a classificação pro helper
// compartilhado — não decide o status, só qual sentinel usar se cair fora do 2xx/4xx.
resp, err := upstream.Do(c.http, req, domain.ErrGameRegistryUnavailable)

// handler (internal/transport/http_handler.go): não sabe qual upstream foi,
// só sabe traduzir UpstreamError (repassa o status) e os sentinels de
// indisponibilidade (-> 502). Handlers novos devem terminar assim.
func (h *Handler) games(w http.ResponseWriter, r *http.Request) {
    list, err := h.svc.Games(r.Context())
    if err != nil {
        writeUpstream(w, err) // *UpstreamError -> mesmo status; sentinel -> 502; resto -> 500
        return
    }
    httpkit.WriteJSON(w, http.StatusOK, list)
}
```

Regra prática ao adicionar endpoint novo: o client upstream usa `upstream.Do(...)` com o sentinel `Err<Serviço>Unavailable` (declarar em `internal/domain/` se ainda não existir); o handler novo termina em `writeUpstream(w, err)` — não reimplementar `errors.Is` a mão (isso é o padrão antigo, pré-`UpstreamError`, que ainda aparece em alguns handlers como `players`/`liveMetrics` mas não deve ser copiado).

Alguns handlers agregam **mais de um upstream** numa resposta só (ex. `gameSessions` enriquece sessões do `session-service` com handle de jogador via `identity`) — nesse caso o primeiro erro que aparecer decide o status; não há "erro parcial" no formato de resposta.

## Rotas

Registradas em `internal/transport/http_handler.go` (`Register`), hoje **43 rotas** — número que muda quase toda semana; a lista completa e atualizada é o próprio código, não este README. Áreas:

- `GET /api/topology`, `GET /api/metrics/live` — saúde/contadores do painel geral (Prometheus).
- `GET /api/players`, `GET /api/players/{playerID}`, `GET /api/players/{playerID}/games` — User base (identity) + presença por jogo.
- `GET|POST|PUT|DELETE /api/games/*` — CRUD de jogos, fleet, descrição, perfil/modos multiplayer nomeados, mode-snapshot-hashes (game-registry).
- `GET|POST|PUT|DELETE /api/games/{gameId}/achievements/*`, `.../event-rules/*` — catálogo, progresso, import CSV, regras de evento (achievements).
- `GET|PUT /api/games/{gameId}/leaderboard/*` — placares (leaderboard-service).
- `GET /api/games/{gameId}/players/*` — player base do jogo, player data, cloud save (identity/playerdata/cloudsave).
- `GET /api/games/{gameId}/sessions`, `GET /api/rooms`, `POST /api/rooms/{matchId}/close` — sessões por jogo e board global (session-service).
- `GET /api/analytics/overview`, `GET /api/analytics/games/{gameId}` — analytics-service.
- `GET /api/health/fanout` — sinais de saúde do fan-out durável (ADR-0020), via Prometheus.

Ao adicionar uma rota: registrar em `Register`, escrever o handler terminando em `writeUpstream` (ver seção acima), e não tentar manter esta lista exaustiva — ela está aqui só pra orientar em qual arquivo/área procurar.

## Observabilidade

Métricas RED em `/metrics` (`middleware.NewMetrics`) + runtime Go. Tracing via **otelhttp** tanto no server (com `RouteSpanName`, exceto `/healthz` e `/metrics`) quanto nos clients de saída — fecha a visão cross-service no Tempo (ADR-0014).

## Testar / Specs

- Manual (HTTP): [`api/README.md`](api/README.md) — exemplos com `curl`
- Specs: [`specs/`](specs/) — 22 specs agrupadas por área:
  - **Overview/topologia:** `001-admin-overview`, `002-service-metrics`
  - **Players:** `003-players`, `006-player-tabs`, `020-player-by-id`
  - **Games:** `004-games`, `010-game-rename`, `018-game-delete-passthrough`
  - **Multiplayer profiles/modos:** `012-multiplayer-profile`, `014-host-world-rooms`, `015-battle-royale-placement`, `016-coop-mission-mission`, `017-multiplayer-modes-passthrough`, `022-mode-snapshot-icon`
  - **Achievements:** `011-achievements-csv-import`, `021-unlocked-at-zero-date-fix`
  - **Leaderboard:** `005-leaderboard-panel`, `019-leaderboard-windows`
  - **Sessions/rooms:** `007-sessions-panel`, `008-rooms`
  - **Analytics:** `009-analytics`
  - **Passthrough genérico:** `013-board-passthrough`
