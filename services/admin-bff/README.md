# admin-bff

Back-end-for-frontend do **painel de administração** ("sala de controle" do backend de jogos — ADR-0016). Agregador **read-only e stateless**: lê o **Prometheus** (saúde + contadores ao vivo) e a **API admin do achievements** e expõe uma API HTTP/JSON para o `admin-ui` (SPA React, repo/pasta à parte). Responsabilidade distinta do Grafana: produto/admin, não séries/infra.

- **Protocolo:** REST (`net/http`) · **Porta host:** 8088 (HTTP: API, `/metrics`, `/healthz`)
- **Storage:** nenhum (não persiste — agrega outras fontes)
- **Dependências:** Prometheus (`PROMETHEUS_URL`), achievements-service (`ACHIEVEMENTS_URL`)
- **Auth:** nenhuma no MVP (interno/local; auth real é evolução — provável ADR futuro)

## API

| Método | Rota | Painel |
|---|---|---|
| GET | `/api/topology` | nós (serviços) com up/down + arestas de dependência |
| GET | `/api/metrics/live` | contadores ao vivo (fila, sessions, matches) |
| GET | `/api/services/{id}/metrics` | saúde + RED + runtime de um serviço (página de domínio) |
| GET | `/api/players` | lista de usuários (User base) — via identity `/admin/players` |
| GET / POST | `/api/games` | listar / criar jogos — via game-registry |
| GET / POST | `/api/games/{gameId}/achievements` | catálogo do jogo / cadastrar conquista — via achievements |
| GET | `/api/games/{gameId}/players/{playerID}/achievements` | achievements do jogador no jogo |

**Degradação por fonte:** Prometheus fora -> `/api/topology` retorna nós `up:null` + `sourceAvailable:false` (200); `/api/metrics/live` e `/api/services/{id}/metrics` -> 503. `id` desconhecido -> 404. achievements fora -> `/api/players/{id}/achievements` -> 502.

**RED por serviço:** vem de `http_requests_total`/`http_request_duration_seconds`, que só os serviços REST expõem. Para os gRPC (matchmaking, session) os campos RED vêm `null` (só `up` + runtime). Contadores de produto (fila/sessions/matches) ficam no `/api/metrics/live`.

## Fontes de dados (ADR-0016)

Nunca acessa banco de outro serviço. Lê:
- **Prometheus** (`/api/v1/query`): `up{job=...}` para saúde; `matchmaking_queue_size`, `session_active_sessions`, `matchmaking_matches_total`/`_failed_total` para os contadores.
- **achievements-service** (`/admin/players/{id}/achievements`): endpoint admin interno (achievements/003).

A topologia (nós + arestas) é estática no MVP (dependências conhecidas — ver `internal/domain/topology.go`).

## Observabilidade

Métricas RED em `/metrics` (`middleware.NewMetrics`) + runtime Go. Tracing via **otelhttp** (server e clients de saída), fechando a visão cross-service no Tempo (ADR-0014).

## Testar / Specs

- Manual (HTTP): [`api/README.md`](api/README.md) — exemplos com `curl`
- Specs: [`specs/`](specs/) — `001-admin-overview`
