# Spec: 001-admin-overview

**Serviço:** admin-bff
**Status:** Approved
**Data:** 2026-06-29

## Contexto

Primeiro corte do `admin-bff` — o back-end-for-frontend do painel de administração ("sala de controle" do backend de jogos), decidido no **ADR-0016**. Ele agrega dados para o `admin-ui` (SPA React, fora deste serviço) a partir de **duas fontes**: a **HTTP API do Prometheus** (estado ao vivo e saúde) e as **APIs dos serviços** (entidades de produto). **Nunca** lê o banco de outro serviço (ADR-0015/0016).

É **read-only** no MVP e **control plane interno** — sem auth no MVP (consistente com session/matchmaking; auth real é evolução, provável ADR futuro). MVP cobre três painéis (ADR-0016): **topologia + saúde**, **contadores ao vivo** e **achievements por jogador**. O painel de **jogadores** fica para a iteração 2 (depende de endpoint novo no identity).

## Comportamento

### API HTTP (consumida pelo admin-ui)

#### Topologia e saúde dos serviços
- WHEN `GET /api/topology` -> retorna os **nós** (serviços Go do backend) com status `up`/`down` e as **arestas** de dependência conhecidas (ex.: matchmaking→session, matchmaking→identity).
- THE SYSTEM SHALL derivar o status de cada nó da métrica `up{job=...}` do Prometheus (1 = up, 0/ausente = down).
- IF o Prometheus estiver indisponível -> retorna os nós com status `unknown` e um campo sinalizando a fonte indisponível, **sem** falhar a resposta inteira (degrada).

#### Contadores ao vivo
- WHEN `GET /api/metrics/live` -> consulta o Prometheus e retorna os contadores operacionais de produto: fila do matchmaking (`matchmaking_queue_size`), sessions ativas (`session_active_sessions`), matches formados (`matchmaking_matches_total`) e falhos (`matchmaking_matches_failed_total`).
- IF uma métrica não existir/ainda não foi exposta -> o campo correspondente vem como `null`/ausente, sem falhar os demais.
- IF o Prometheus estiver indisponível -> `503` com corpo de erro claro (é a única fonte deste painel).

#### Achievements por jogador
- WHEN `GET /api/players/{playerID}/achievements` -> chama o endpoint admin do achievements-service e retorna o **catálogo anotado** com o progresso daquele jogador.
- IF o jogador não tem progresso -> retorna o catálogo com progresso zero / `unlocked=false` (não é erro).
- IF o achievements-service estiver indisponível -> `502` com erro claro.

#### Saúde e métricas do próprio BFF
- THE SYSTEM SHALL expor `GET /healthz` e `GET /metrics` (RED + OpenMetrics), como todo serviço (ADR-0006).

### Invariantes
- THE SYSTEM SHALL agregar **somente** via Prometheus e APIs de serviço; **nunca** acessar diretamente o banco de outro serviço (ADR-0016).
- THE SYSTEM SHALL tratar cada fonte de forma isolada: a indisponibilidade de uma fonte degrada apenas o painel que depende dela, não a API inteira.

## Contratos de Interface

### API HTTP do BFF
| Método | Rota | Retorno |
|---|---|---|
| GET | `/api/topology` | `{ nodes: [{id,label,up}], edges: [{from,to,label}], sourceAvailable }` |
| GET | `/api/metrics/live` | `{ queueSize, activeSessions, matchesTotal, matchesFailed }` |
| GET | `/api/players/{playerID}/achievements` | `[{ id,name,description,target,progress,unlocked,unlockedAt }]` |
| GET | `/healthz`, `/metrics` | saúde / métricas do próprio serviço |

### MetricsSource (Port de saída)
- `Query(ctx, promQL string) ([]Sample, error)` — consulta instantânea à HTTP API do Prometheus (`/api/v1/query`).

### AchievementsReader (Port de saída)
- `ForPlayer(ctx, playerID string) ([]PlayerAchievement, error)` — lê o endpoint admin do achievements-service.

### Tipos (domínio)
- `ServiceNode{ ID, Label string; Up *bool }` — `Up` nil = desconhecido (fonte indisponível)
- `Edge{ From, To, Label string }`
- `Topology{ Nodes []ServiceNode; Edges []Edge; SourceAvailable bool }`
- `LiveMetrics{ QueueSize, ActiveSessions, MatchesTotal, MatchesFailed *float64 }`
- `PlayerAchievement{ ID, Name, Description string; Target, Progress int; Unlocked bool; UnlockedAt time.Time }`
- `Sample{ Labels map[string]string; Value float64 }`

## Casos de Teste Obrigatórios

- [ ] `GET /api/topology` -> nós com `up` derivado de um Prometheus fake (job up=1 -> up; ausente -> down)
- [ ] Prometheus indisponível -> `/api/topology` retorna nós `unknown` + `sourceAvailable=false`, status 200 (degrada)
- [ ] `GET /api/metrics/live` -> agrega os contadores a partir do Prometheus fake
- [ ] Prometheus indisponível -> `/api/metrics/live` -> 503 com erro
- [ ] `GET /api/players/{id}/achievements` -> repassa o catálogo anotado de um achievements fake
- [ ] achievements indisponível -> `/api/players/{id}/achievements` -> 502 com erro
- [ ] As arestas de topologia refletem as dependências conhecidas (matchmaking→session, matchmaking→identity)

## Fora do escopo

- **Painel de jogadores** (listar/contar players) — iteração 2; depende de endpoint novo no identity
- **Ações administrativas** (banir, forçar fim de sessão, reset) — read-only no MVP
- **Auth** (do BFF e dos endpoints admin de serviço) — interno/local no MVP; provável ADR futuro
- **Painéis de leaderboard / cloud-save / player-data / analytics** — incrementais depois
- **O `admin-ui`** (front React) — app à parte, documentação leve, fora do ciclo SDD (ADR-0016)
- **Listas de runtime detalhadas** (quais salas/sessions, com quem) — exige endpoints admin nos serviços gRPC (iteração 2)
