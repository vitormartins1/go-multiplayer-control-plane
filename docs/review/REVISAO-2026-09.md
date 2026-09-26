# Revisão de produto e de código — control plane (2026-09)

> Pedido: `docs/prompts/PROMPT-REVISAO-PRODUTO.md`. Sessão autônoma na nuvem (VM Linux, sem Unity, sem
> Agones, sem o Docker Desktop do dono), em 2026-09-26, sobre a `main` em `0facdf5`.
> Lentes: **(F)** Folclore Souls · **(G)** produto genérico · **(V)** vitrine de Go/engenharia.
> Progresso da execução (Fase 2): [`PROGRESSO.md`](PROGRESSO.md).

## Resumo em 10 linhas

1. O núcleo que o Folclore Souls usa (identity → game-registry/modos → matchmaking de invasão/sinais → session com
   Agones fleet-aware → fan-out durável → achievements/analytics) **existe, é testado e está bem desenhado**.
2. **A `main` estava vermelha**: 1 teste do admin-bff entra em pânico desde `c8168bb` (spec 088) — o CI falharia.
3. **Bug real de perda de evento**: um `HOST_KILLED` cujo publish falha nunca mais é publicado (o mundo já foi
   apagado; o retry recebe `NotFound`, que o contrato manda o server tratar como "pare").
4. **Segurança é a maior lacuna genérica**: o gRPC do session não tem autenticação nenhuma (qualquer um abre
   mundo, entra como qualquer `player_id` e reporta resultados que viram conquista/ranking), e o `/admin` do
   session e do analytics estão fora do guard da internal key que todo o resto usa.
5. A documentação de estado está **dois meses atrás do código** (NEXT-STEPS de 07-13, README dizendo "rename não
   executado", "Go 1.22+", session "em memória", sinais dizendo ADR-0026 "Proposed" quando está implementada).
6. O **admin-ui é o maior custo do repo**: 14,1k LOC de TSX (≈ 67% de todo o Go de produção), **zero testes**,
   88 specs das quais ~40 são cosméticas; partes exibem **dados fabricados** (Fleet, Infrastructure).
7. `battle_royale`/`coop_mission` custam código em 6 serviços sem nenhum jogo real — congelar, não expandir.
8. O Go é idiomático e disciplinado (hexagonal real, `ctx` em todo I/O, erros com contexto, shutdown gracioso
   nos 12 binários, lint limpo); as dívidas são pontuais (cobertura de analytics 61%, `session_service.go` com
   1.075 linhas, boilerplate de `main.go` copiado).
9. Para a vitrine falta a **primeira impressão**: README atual não tem diagrama, quickstart verificável nem demo.
10. Executei na Fase 2 os itens que não dependem de decisão (ver §5 e `PROGRESSO.md`), com uma demo do fluxo do
    Folclore Souls validada ponta a ponta; ficam 6 decisões (§6) e uma pendência de aval (§6, "shared/").

---

## 1. Estado verificado

### 1.1 Medições (2026-09-26)

| Medida | Valor | Como |
|---|---|---|
| Go de produção (sem `*.pb.go`) | ~21,1k LOC | `wc -l`, excluindo `_test.go` |
| Go de teste | ~20,4k LOC · 1.067 `func Test` | idem |
| admin-ui (TS/TSX em `src/`) | ~14,1k LOC em 64 arquivos · **0 testes** | `wc -l`; `package.json` não tem script `test` |
| Specs SDD | 216 (admin-ui 88, admin-bff 28, session 28, achievements 18, analytics 15, game-registry 13, matchmaking 11, leaderboard 7, identity 4, cloud-save 2, player-data 2) | `ls services/*/specs` |
| `go build ./...` · `go vet ./...` | ✅ limpos | Go 1.26.0 via `GOTOOLCHAIN=auto` |
| `golangci-lint run ./...` (v2.12.2, config do repo) | ✅ 0 issues | `.golangci.yml` só liga os linters default |
| `go test ./... -race` | ❌ **50 pacotes ok, 1 FAIL** | `TestGameLeaderboard_DelegaAoLeaderboard` — nil pointer em `services/admin-bff/internal/service/admin_service.go:441` |
| `go test -tags=integration ./...` (testcontainers, Docker da VM) | ❌ 55 ok, 3 FAIL | (a) o do admin-bff acima; (b) **3 testes do analytics com data fixa** (`2026-07-13`) consultados com janela de 30 dias — passaram em julho e quebraram sozinhos com o tempo (`analytics-service/internal/repository/postgres_analytics_test.go:833,852,868`); (c) cloud-save: a VM não conseguiu baixar `minio/minio:latest` (`pull access denied`; `quay.io` 403) — não sei se é a rede da VM ou a imagem saiu do Docker Hub: **validar localmente** |
| admin-ui `tsc --noEmit` + `vite build` | ✅ | chunks maiores: mantine 475 kB, DonutChart 411 kB, PlayerAvatar 337 kB, dicebear 194 kB, xyflow 183 kB |
| Dashboards Grafana | 10 JSON provisionados | `infra/docker/grafana/provisioning/dashboards/` |
| Terraform | só `infra/terraform/environments/local/messaging/` | não há `prod/` nem Terraform por serviço |

Cobertura `internal/service` / `internal/domain` (regra da casa: ≥ 80%):

| Serviço | service | domain |
|---|---|---|
| session | 95,0% | 100% |
| matchmaking | 92,6% | 97,0% |
| leaderboard | 91,3% | — |
| player-data | 90,0% | — |
| game-registry | 89,5% | — |
| identity | 87,9% | — |
| cloud-save | 100% | — |
| achievements | **77,7%** | 94,6% |
| analytics | **60,9%** | — |
| admin-bff | (FAIL) | — |

### 1.2 O que existe e funciona — por lente

**(F) O que o Folclore Souls usa**

| Capacidade | Estado | Evidência |
|---|---|---|
| Alocação Agones **por jogo** (ADR-0026) | ✅ implementada | `AGONES_FLEET` não existe mais; `NewAgones(client, namespace)` em `services/session-service/cmd/server/main.go:279-283`; fleet por chamada em `internal/allocator/agones.go:51-61`; vem de `Game.Fleet` (`services/game-registry/internal/domain/game.go:48`, `PUT /games/{id}/fleet`). Sem fleet → `FailedPrecondition "fleet not configured"`. |
| Mundo do host, join, invasão, sinais, senha | ✅ | RPCs em `session.proto` (OpenWorld:24, JoinWorld:29, …) e `matchmaking.proto` (Invade:18, PostSummonSign:24, ListSigns:26, SummonSign:30). |
| Coops em `players[]` (follow-up #15) | ✅ | `JoinWorld` acumula `Players`/`Participants` com dedupe (`session_service.go:834-838`); `DIED`/`LEFT` só liberam o slot (`:970-974`). |
| `stats` no `ReportParticipationEvent` | ✅ ponta a ponta | proto `session.proto:142` → envelope `publisher/sns_publisher.go:95` → achievements (`stat.<chave>` como fact, `achievements-service/internal/domain/event_rule.go:173-176`) → analytics (`repository/postgres_analytics.go:138`). O jogo ainda não preenche. |
| Mutação antes do publish (fix `49e976d`) | ✅, **com bug no `HOST_KILLED`** | `session_service.go:952-985` — ver risco R2. |
| Histórico de sessões em Postgres (session/025) | ✅ fechadas; **mundos abertos seguem só em memória** | `repository/postgres_history.go`; mapa `s.sessions` em `session_service.go:42-43`. Restart do session-service mata todos os mundos vivos. |
| Parâmetros novos do arquétipo sem mudar backend | ✅ já possível | passthrough `worldParams`/`ExtraParams` (follow-up #14, `game-registry/internal/jsonparams`) — chave desconhecida chega ao Unity no `params_json`. Os "9 params propostos" **não precisam de código aqui** até um virar regra do control plane. |

**(G) Produto genérico**

- Multi-tenant por `game_id`, perfis por modo (ADR-0023/0024), soft delete (ADR-0025) — coerentes e testados.
- Fan-out durável SNS→SQS com consumers idempotentes na mesma transação do efeito (analytics/achievements:
  `postgres_analytics.go:38,108`, `postgres_achievement.go:268,327`; leaderboard via Lua `SADD`+`ZINCRBY`,
  `leaderboard-service/internal/storage/redis_store.go:87-131`).
- Todos os `http.Client` de saída têm timeout de 5s; os 12 `cmd/*/main.go` fazem shutdown gracioso
  (`signal.NotifyContext` + `Shutdown`/`GracefulStop`).
- Player-facing HTTP autentica por JWT EdDSA via JWKS (`shared/auth/jwks_verifier.go:68-85`) e o `playerID`
  **sempre vem do token** (sem impersonação por body/path). Senhas com bcrypt + comparação dummy contra timing
  oracle (`identity-service/internal/service/auth_service.go:25,64,86`).

**(V) Vitrine**

- Hexagonal real (nenhum `service/` importa `transport/`/`repository/`), `ctx` em todo I/O, `slog` JSON, erros
  com contexto, métricas RED + OTel + `otelpgx` em todos os serviços com Postgres, 10 dashboards.
- `docs/GO-LEARNINGS.md` (1.323 linhas) e 29 ADRs são um diferencial raro — mostram raciocínio, não só código.

### 1.3 Onde a documentação diverge do código

| Doc | Afirma | Realidade |
|---|---|---|
| `README.md:5` | "rename ainda não executado" | módulo e remoto já são `go-multiplayer-control-plane` |
| `README.md:38` | session: storage "memória" | histórico em Postgres próprio (`session-postgres`) |
| `README.md:46`, `CLAUDE.md:9` | Go 1.22+ | `go.mod`: `go 1.26.0` |
| `CLAUDE.md:40`, `AGENTS.md:9`, `ARCHITECTURE.md:143` | Terraform prod por serviço (`environments/prod/identity-service`) | não existe; só `environments/local/messaging` |
| `CLAUDE.md:16`, `ROADMAP.md:39-40` | CI com deploy seletivo por path | um workflow único (build/vet/test/integração/lint/admin-ui build), sem path filter nem deploy |
| `CLAUDE.md:11` | session com gRPC streaming | session é só unário; streaming só em `FindMatch`/`PostSummonSign` |
| `ARCHITECTURE.md:23-33,76-86` | mapa de serviços | omite o game-registry |
| `ARCHITECTURE.md:167,204` | JWKS no identity; RPC `AllocateSession` | JWKS do connect token é servido pelo session; o RPC é `CreateSession` |
| `NEXT-STEPS.md` (inteiro) | estado de 2026-07-13 | não cobre analytics 012–015, session 027/028, admin-bff 025–028, admin-ui 063–089, fix `49e976d`, slug `7f174a4`; fala de dashboard configurável (revertido pela spec 067), "7 dashboards" (são 10), "4 workers de fila" (são 2 binários consumer + o consumer embutido no analytics), ADR-0026 "Proposed" e "Accepted" no mesmo arquivo |
| `SINAIS-DO-FOLCLORE-SOULS.md:32` | ADR-0026 Proposed, `AGONES_FLEET` fixo | implementada (ver 1.2) |
| `host_world_dropin-dataplane-contract.md:123` + `session_service.go:932-934` + `session.proto:43` | "publica ANTES de mutar" | desde `49e976d` muta antes e publica depois |
| contrato `:132-145` | envelope sem `modeId`/`eventId`; sinks idempotentes por `(sessionId, playerId, event)` | envelope carrega `modeId`/`eventId`; idempotência é por `eventId` |
| contrato (erros) | só `InvalidArgument`/`NotFound` | também `FailedPrecondition` (arquétipo/fleet) e `Internal` |
| `OBSERVABILITY.md:48,91-94` | 9 dashboards; worker `analytics-consumer` | 10; não há esse job no `prometheus.yml` |
| `OBSERVABILITY.md:105` × `PRODUCT-OVERVIEW.md:69` | tracing até o Dedicated Server "futuro" × "já atravessa" | contradição entre os dois docs |
| `PRODUCT-OVERVIEW.md:20`, `NEXT-STEPS.md:80,82`, `host_world_dropin-follow-ups.md`, `multiplayer-modes-roadmap.md:58` | "Capoeira Souls" | Folclore Souls desde `7f174a4` |
| `game-models/README.md:7,56,93,114,130` | "um arquétipo só", BR squads "em aberto", leaderboard cumulativo "planejado" | os 4 arquétipos completos; matchmaking/008 e leaderboard/004 feitos |
| `game-registry/README.md:30,35` | "slug/id estáveis" | `PUT /games/{id}/slug` (`7f174a4`) — **sem spec** (lacuna SDD) |
| specs com status/checkbox velho | admin-bff/028, admin-ui/069-070, analytics/015 "Proposed"; matchmaking/008 "Rascunho"; tasks de session/005, admin-bff/007, admin-ui/007 sem `[x]` | todas implementadas e mergeadas |

---

## 2. O que cortar ou simplificar

Critério: custo de manter (LOC, specs, superfície de bug, dependências) × valor para um jogo real ou para a
vitrine. "Congelar" = não evoluir sem caso real, mas manter verde.

| Item | Custo | Valor | Recomendação |
|---|---|---|---|
| **Dados fabricados no console**: Fleet (`mockFleetStatus`, `mockFleetHealth`, `MOCK_AGONES_CLUSTERS` — `admin-ui/src/pages/Fleet.tsx:59-138`) e cards de custo "Go × AWS" com valores fixos (`Overview.tsx:245-324`, `SERVICE_META` "ILUSTRATIVO" `:55-66`) | ~350 LOC, 5 specs de redesign (077–081) | negativo para a vitrine: número inventado numa tela de operação mina a credibilidade do que é real | **Remover** as zonas mock (manter só o que vem do cluster/Prometheus); a tese de custo vai para um doc com a medição quando existir. *Decisão D4.* |
| **Rotas mortas**: `PUT /api/games/{id}/multiplayer-profile` (legado pré-modos, `admin-bff/internal/transport/http_handler.go:93`, cliente `setMultiplayerProfile` com 0 chamadas) e `GET /api/games/{id}/multiplayer-profiles/{modeId}` (`:97`, sem cliente) | 2 rotas + testes | nenhum consumidor | **Remover** (executado na Fase 2, item E6). |
| Endpoints de backend sem chamador: achievements `POST /admin/games/{g}/players/{p}/events`, leaderboard `POST /admin/.../scores`, game-registry `GET /games/{id}` | pequenos | ferramentas de operação/teste manual | **Congelar** (baratos; úteis para smoke manual). |
| `@playwright/test` no `package.json` sem teste nem config | dependência pesada | nenhum hoje | **Remover** ou usar de verdade (ver lacuna V3). *Junto com D4.* |
| **Editor visual de regras** (React Flow, ~3,6k LOC, 16 specs 043–058) | 25% do admin-ui, gotcha documentado de alturas estimadas | é o diferencial de produto mais citado; o Folclore usa event-rules | **Congelar** — nenhuma feature nova no canvas sem pedido de jogo; a view Tabela é o caminho de manutenção barata. |
| Layout do Analytics (7 specs de layout, 062 revertida pela 067) | churn alto | dados reais, mas o layout já mudou 7× | **Congelar** o layout; só entra métrica nova se um jogo pedir. |
| DiceBear (avatares/capas geradas, ~530 kB de chunks) + setting de estilo de capa | peso de bundle, 1 setting | cosmético | **Simplificar** quando mexer no admin-ui (capa/iniciais em CSS). Não urgente. |
| `battle_royale` e `coop_mission` (contrato, perfis, consumers, rollups, UI em 6 serviços) | código vivo em session, matchmaking (lobby BR), achievements, leaderboard, analytics, admin-ui | **nenhum jogo real** — o próprio dono já aprendeu que isso vira especulação | **Congelar**: manter verde, marcar como "experimental" nos docs, nada novo até existir um jogo. Remover custaria mais que manter. *Decisão D5.* |
| Specs de polimento do admin-ui (~40 das 88) | burocracia | registro histórico | Não apagar; **parar de abrir spec para ajuste cosmético** (commit `fix(admin-ui)` direto, como o SDD de dois níveis já permite). |

Não recomendo cortar serviços inteiros: leaderboard, cloud-save e player-data são pouco usados pelo Folclore
hoje, mas são pequenos (2–7 specs cada), têm cobertura ≥ 87% e são exatamente as peças que o "progresso do mundo"
(decisão em aberto do jogo) vai precisar.

---

## 3. O que falta

### 3.1 Para o Folclore Souls (F)

| # | Lacuna | Estado no código | Observação |
|---|---|---|---|
| F1 | **`HOST_KILLED` perdido em falha de publish** | bug (R2) | corrigido na Fase 2 (E2). |
| F2 | **Bearer obrigatório no `OpenWorld` e host derivado do token** | não existe: gRPC do session sem interceptor (`session-service/cmd/server/main.go:181-183`); `OpenWorld` confia em `host_player_id` (`transport/grpc_server.go:126`) | o jogo já extrai o `sub` do JWT localmente. Mudança de contrato → *Decisão D1*. |
| F3 | **Autenticar o game server** (`ReportParticipationEvent`, `ReportResult`, `GetSessionConfig`) | nenhum | hoje qualquer cliente que alcança a porta 50052 reporta resultado que vira conquista/ranking. *D1.* |
| F4 | **Identidade do boss no `BOSS_DEFEATED`** | sem campo; **workaround funciona hoje**: `stats: {"boss_boitata": 1}` vira fact `stat.boss_boitata` nas event-rules e é gravado no analytics | recomendo convenção de `stats` (zero mudança de proto) até o analytics precisar de dimensão "boss". *Decisão D3.* |
| F5 | Invasão **não filtra por modo** | `ListInvadableWorldsRequest` só tem `game_id` (`session.proto:115-117`); o `mode_id` do invasor só escolhe a política (`matchmaking-service/internal/service/invader.go:68`) — um invasor do modo A pode cair num mundo do modo B; o quadro de sinais já é por modo | compatível (campo opcional), mas muda quem invade quem. *Decisão D2.* |
| F6 | Mundos abertos só em memória | restart do session-service derruba todos os mundos vivos | com restart policy (E4) o serviço volta, mas vazio. Persistir mundos abertos só quando houver jogador real (fase multiplayer do jogo). |
| F7 | Reconexão de host/phantom | não existe (`ErrAlreadyJoined`, `session_service.go:822-825`; contrato diz "sem reconnect") | esperar o jogo voltar a multiplayer; desenho depende do netcode do jogo. |
| F8 | Alocação sob demanda (follow-up #10) | `OpenWorld` aloca na hora (`session_service.go:754`) | é a economia real para um Souls-like; depende de topologia (P2P × dedicado) que o jogo ainda está decidindo. |
| F9 | Fila de espera de invasão / pools por região | `Invade` é unário, rebusca no máximo 2× (`invader.go:60,77-96`); não há área/região | só com jogadores reais. |
| F10 | Progresso do mundo (checkpoint, bosses) | cloud-save/player-data existem e não são usados | decisão do jogo; o backend já tem os dois caminhos. |

### 3.2 Para um produto genérico sério (G)

| # | Lacuna | Evidência |
|---|---|---|
| G1 | `/admin` do **session** e do **analytics** fora do guard da internal key (todo o resto usa `RequireInternalKey`) — inclui `POST /admin/sessions/{id}/close` | `session-service/cmd/server/main.go:210`; `analytics-service/cmd/server/main.go:116-124`. **Corrigido na Fase 2 (E3).** |
| G2 | **admin-bff sem autenticação**: `/api/*` repassa com a internal key, então quem alcança a porta 8089 apaga jogo, muda score etc. | `admin-bff/internal/transport/http_handler.go:81-129`. Aceitável em dev; bloqueador para qualquer deploy. *Decisão D6.* |
| G3 | JWT sem checagem de `iss`/`aud`; o jogo vem do header `X-Game-Id` não autenticado (token de um jogo vale em todos) | `shared/auth/jwks_verifier.go`; `cloud-save/internal/transport/http_handler.go:136` |
| G4 | `kid` desconhecido dispara fetch de JWKS a cada request (sem singleflight/throttle) — amplificação contra o identity | `shared/auth/jwks_verifier.go:91-104` (mexe em `shared/` → precisa de aval) |
| G5 | Sem rate limit/lockout no login; `MaxBytesReader` só no cloud-save (player-data gravava valores sem teto); sem CORS explícito. *(Os timeouts de servidor estão ok: todos os `http.Server` têm `ReadTimeout`, e o `IdleTimeout` herda dele.)* | `identity-service`, `player-data-service`. **Teto de corpo corrigido na Fase 2 (P4).** |
| G6 | Identity sobe com chave de assinatura **efêmera** no compose (tokens morrem a cada restart) | `identity-service/cmd/server/main.go:168-172`; compose só passa chave ao session. **Corrigido na Fase 2 (P5).** |
| G7 | `ReportResult` sem outbox: crash entre publish e close deixa sessão aberta já publicada; retry após sucesso dá `NotFound` | `session_service.go:305-352` |
| G8 | Set de idempotência do leaderboard sem TTL (cresce para sempre) | `redis_store.go:87-131`. **Corrigido na Fase 2 (spec `leaderboard/008`).** |
| G9 | Nenhum serviço do compose tinha restart policy (lição do incidente de 2026-08-19 não aplicada) | só `localstack` (`docker-compose.yml:421`). **Corrigido na Fase 2 (E4).** |
| G10 | Deploy real/medição de custo (a tese do ADR-0008) não começou | só Terraform local de mensageria |

### 3.3 Para a vitrine (V)

| # | Lacuna |
|---|---|
| V1 | **`main` vermelha** (teste do admin-bff). Corrigido na Fase 2 (E1). |
| V2 | README de primeira impressão: sem diagrama, sem quickstart verificável, dados errados (§1.3). Reescrito na Fase 2 (E5). |
| V3 | admin-ui sem nenhum teste nem lint (14k LOC). Mínimo sério: Vitest nos `lib/` puros + ESLint no CI. **Vitest + 12 testes no CI na Fase 2 (P2); ESLint e `rulesGraph.ts` ficam para depois.** |
| V4 | Cobertura abaixo da régua da casa: analytics 60,9%, achievements 77,7%. **Corrigido (E7): 92,8% e 93,4%.** |
| V5 | Demo reproduzível: não há script que exercite o fluxo `login → OpenWorld → Invade → ReportParticipationEvent → conquista`. **Feito (P1): `scripts/demo-host-world.sh`, validado ponta a ponta.** |
| V6 | CI sem `govulncheck`, sem build de imagem, sem gate de cobertura; lint só com linters default. |
| V7 | `session_service.go` com 1.075 linhas (4 arquétipos num tipo só); boilerplate de `main.go` (pgxpool, healthz, metrics) copiado em 7–12 binários; middleware `auth`/`playerID`/`gameHeader` copiado em 4 serviços. Refatorar **só** com teste cobrindo — é o que um revisor de Go notaria primeiro. |
| V8 | 44 `_ =` em código de produção; os que importam: `json.Unmarshal` ignorado em `game-registry/internal/transport/http_handler.go:193,202,216,225`. **Os do game-registry corrigidos na Fase 2 (400 em vez de zero-value).** |

---

## 4. Riscos e dívida técnica

| # | Risco | Severidade | Situação |
|---|---|---|---|
| R1 | **gRPC do session sem autenticação** e publicado no host (`50052`): impersonação de jogador, resultado forjado alimenta conquistas/ranking, abrir mundos à vontade aloca GameServers (custo) | **Alta** (bloqueia qualquer deploy público) | D1 |
| R2 | **`HOST_KILLED` perdido**: o mundo é fechado e removido do mapa antes do publish (`session_service.go:975-980`); o retry do server cai no lookup inicial e recebe `NotFound` ("pare"); `invasion_won` do invasor some. Idem se o `Release` do Agones falhar | **Alta** para o Folclore (evento de gameplay central) | Corrigido (E2) |
| R3 | `main` vermelha desde `c8168bb` | Média (confiança/vitrine) | Corrigido (E1) |
| R4 | `/admin` do session/analytics sem guard | Média | Corrigido (E3) |
| R5 | admin-bff sem auth | Média em dev, **Alta** em deploy | D6 |
| R6 | Mundos vivos só em memória + serviços sem restart policy | Média | restart policy corrigida (E4); persistência adiada |
| R7 | Docs de estado desatualizadas → decisões erradas (o próprio documento de sinais pediu uma feature já feita) | Média | Corrigido (E5) |
| R8 | admin-ui sem testes: 3,6k LOC de canvas com layout por altura estimada | Média | V3 |
| R9 | Invasão cruzando modos | Baixa hoje (Folclore tem poucos modos), Média com mais modos | D2 |
| R10 | Crescimento sem limite do set de idempotência do leaderboard | Baixa | Corrigido (`leaderboard/008`) |
| R11 | JWKS fetch amplificável | Baixa (dev), Média (prod) | G4 |

---

## 5. Roadmap priorizado

Ordem por valor/esforço. **E** = executo nesta sessão (não depende do dono). **D** = depende de decisão (§6).
**P** = próximo, executável sem decisão, fica para a próxima sessão.

| # | Item | Lente | Impacto | Esforço | Feito quando | Status |
|---|---|---|---|---|---|---|
| E1 | Corrigir o teste do admin-bff (`main` verde) e cobrir o enriquecimento de handle do leaderboard | V | Alto | XS | `go test ./... -race` verde; teste afirma o handle | **feito** |
| E2 | `HOST_KILLED`/evento de mundo recém-fechado: retry publica em vez de `NotFound` (spec `session-service/029`) | F | Alto | S | teste reproduz a perda e passa; contrato e comentários corrigidos | **feito** |
| E3 | `RequireInternalKey` no `/admin` do session e do analytics | G | Alto | XS | teste de transporte/401 sem chave; BFF segue funcionando (já envia a chave) | **feito** |
| E4 | `restart: unless-stopped` nos serviços de longa duração do compose | F/G | Médio | XS | `docker compose config` válido | **feito** |
| E5 | Sincronizar docs de estado: README (reescrito p/ primeira impressão), NEXT-STEPS (reescrito), SINAIS (ADR-0026), contrato HWD (ordem, envelope, erros, idempotência), Capoeira→Folclore nos docs vivos, CLAUDE/AGENTS (Go 1.26, Terraform real), specs com status velho, spec retroativa do slug | V/F | Alto | S | nenhuma divergência da §1.3 restante | **feito** |
| E6 | Remover as 2 rotas mortas do admin-bff + cliente sem uso no admin-ui | V | Baixo | XS | build/testes verdes; `tsc` ok | **feito** |
| E7 | Cobertura do analytics `internal/service` ≥ 80% e achievements ≥ 80% | V | Médio | S | `go test -cover` ≥ 80% | **feito** |
| P1 | Demo reproduzível: `scripts/demo-host-world.sh` (grpcurl + curl do fluxo host_world) | V | Alto | M | passa de ponta a ponta contra a stack | **feito** (`5a6ebbb`) |
| P2 | Vitest nos `lib/` puros do admin-ui + ESLint no CI | V | Médio | M | CI roda testes do front | **Vitest feito** (`c57abe0`); ESLint próximo |
| P3 | TTL no set de idempotência do leaderboard | G | Baixo | S | teste de expiração | **feito** (`5e2aaf2`, spec 008) |
| P4 | Teto de corpo (`MaxBytesReader`) nas rotas de jogador | G | Médio | S | por serviço, sem tocar `shared/` | **feito** (`2690675`, spec `player-data/003`) |
| P5 | Chave de assinatura persistente do identity no compose (secret como o do session) | G | Médio | XS | tokens sobrevivem a restart | **feito** (`092b1eb`) |
| P6 | `govulncheck` no CI | V/G | Médio | XS | job verde | próximo — não validável na VM (`vuln.go.dev` bloqueado) |
| D1 | Auth no gRPC do session (bearer do jogador em `OpenWorld`/`JoinWorld`; credencial do game server nos reports) | F/G | **Alto** | M | contrato v2 + jogo atualizado | decisão |
| D2 | Filtrar invasão por modo (`mode_id` opcional em `ListInvadableWorlds`) | F | Médio | S | spec + teste | decisão |
| D3 | Boss no `BOSS_DEFEATED`: convenção de `stats` × campo novo | F | Médio | XS/S | contrato documenta | decisão |
| D4 | Remover dados fabricados do console (Fleet/Infrastructure) + `@playwright/test` | V | Médio | S | nenhum número mock na UI | decisão |
| D5 | Rotular `battle_royale`/`coop_mission` como experimentais e congelar | G | Baixo | XS | docs | decisão |
| D6 | Auth do console (admin-bff) antes de qualquer deploy | G | Alto (para deploy) | M | ADR + implementação | decisão |

---

## 6. Decisões que só o dono pode tomar

> **Decididas em 2026-09-26:** o dono aprovou a revisão e seguiu a recomendação em todas — D1 (b) em duas fatias com
> compatibilidade, D2 (a), D3 (a), D4 (a), D5 (a), D6 (decidir junto com o deploy) — e liberou mexer em
> `shared/auth` (G3/G4). Execução acompanhada em [`PROGRESSO.md`](PROGRESSO.md) (Fase 3).

**D1 — Autenticação no session gRPC (contrato com o jogo).**
Opções: (a) manter como está até a fase multiplayer voltar; (b) **bearer do jogador obrigatório em
`OpenWorld`/`JoinWorld`** (host/`player_id` derivados do token, como o matchmaking já faz) + **credencial do game
server** (ex.: a internal key, ou um token por GameServer emitido na alocação) para `Report*`/`GetSessionConfig`;
(c) só a (b) do jogador, deixando o server para depois.
**Recomendo (b) em duas fatias, com período de compatibilidade** (aceita sem token com log de aviso, controlado
por env, depois exige). É o maior risco do produto e o jogo já tem o JWT em mãos (`SessionRpcClient.cs`). Muda o
contrato → precisa do seu aval e de uma ADR (evolução da ADR-0018).

**D2 — Invasão respeita o modo?**
(a) filtrar por `mode_id` quando o invasor informa um (campo opcional no `ListInvadableWorldsRequest`, vazio =
comportamento atual); (b) manter cruzado. **Recomendo (a)**: o quadro de sinais já é por modo e o contrato diz que
o modo "flui pelo Invade". Compatível no proto, mas muda quem encontra quem — por isso é seu.

**D3 — Identidade do boss.**
(a) convenção em `stats` (`{"boss_<id>": 1}`, declarando as chaves em `scoringStats` do modo) — funciona hoje nas event-rules e fica gravado no analytics;
(b) campo `subject_id` no `ReportParticipationEventRequest` (compatível) + dimensão no envelope/analytics.
**Recomendo (a) agora** (zero código, o jogo está em fase de gameplay) e (b) quando o analytics por boss for pedido.

**D4 — Dados fabricados no console.** (a) remover as zonas mock da Fleet e os custos fixos da Infrastructure;
(b) manter rotuladas como "mock". **Recomendo (a)**: numa vitrine, um número inventado desvaloriza os reais ao lado.

**D5 — `battle_royale`/`coop_mission`.** (a) congelar e rotular "experimental, sem jogo real"; (b) remover;
(c) seguir evoluindo. **Recomendo (a)** — remover custa mais que manter verde, e o próprio histórico mostra que
genérico sem caso real não se paga.

**D6 — Auth do console.** Antes de qualquer deploy fora da máquina local: (a) login de operador no admin-bff
(OIDC/senha + sessão) ou (b) console só atrás de rede privada/VPN. **Recomendo decidir junto com o G10 (deploy)**;
não faz sentido construir antes.

**Aval pendente (regra da casa, não é decisão de produto):** duas correções de segurança moram em `shared/auth`,
que o `CLAUDE.md` pede para não tocar sem consulta — (a) checar `iss`/`aud` no `JWKSVerifier` e (b) `singleflight` +
intervalo mínimo no refetch de JWKS por `kid` desconhecido (G3/G4). **Recomendo aprovar as duas**: são pequenas e
fecham a amplificação contra o identity.

**Achado da bancada (Fase 2):** com o SNS travado, o `ReportParticipationEvent` ficava pendurado até o cliente
desistir — corrigido com timeout de 5s por publish (`9b58304`, GO-LEARNINGS §38).

