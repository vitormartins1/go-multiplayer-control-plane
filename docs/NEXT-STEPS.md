# NEXT-STEPS — handoff (atualizado 2026-07-13)

Ponto de retomada após limpar o contexto. Resume **o estado atual**, **o que fica pendente de você**, **o que foi feito nesta sessão** e o **backlog priorizado**.

> **Trilha de serviços do control plane: 100% construída e validada — os 4 arquétipos ponta a ponta, motor de achievements/event-rules maduro, Analytics com gráficos.** O que falta de verdade agora é **fora deste repo** (data planes Unity) ou **infra real** (AWS). Ver "Estado desde 2026-07-08" logo abaixo pro que mudou nas últimas sessões.

## Estado desde 2026-07-13 (o que este doc ainda não registrava)

**Analytics Fase 2/3 — FEITO, 5 specs, 2026-07-13 (item 6 do backlog, escopo completo aprovado pelo Vitor):**

- **Achado no meio do caminho, fechado primeiro:** ao especificar captura de
  stats, descobrimos que `achievements/018` ("implementada") só tinha
  fechado o lado de RECEBER stats no `host_world_dropin` — o session-service
  nunca enviava o campo (proto/domínio/envelope sem `stats`). Nenhum
  achievement por stat de HWD jamais disparou de verdade, silenciosamente.
  Fechado por `session-service/026-participation-event-stats`: `stats
  map<string,int64>` em `ReportParticipationEvent`, mesmo padrão dos outros
  3 kinds. Validado ao vivo: `deaths_1` desbloqueou de verdade pela primeira
  vez (jogo "Achievements Test"). Doc do contrato HWD corrigido (a linha do
  checklist Unity estava incorreta/otimista).
- **`analytics-service/010`**: série "por dia" pros 3 arquétipos que só
  tinham rollup cumulativo (battle_royale/coop_mission/host_world_dropin) —
  3 tabelas novas, já com `mode_id`.
- **`analytics-service/011`**: captura de stats brutos por jogador (os 4
  kinds do fan-out, incluindo HWD depois da 026) — tabela nova
  `game_player_stats`, resumo agregado (soma/contagem/média) no endpoint.
- **`admin-bff/024`**: passthrough dos campos novos, sem rota nova.
- **`admin-ui/062`**: dashboard configurável por widget na aba Analytics —
  `react-grid-layout` (API v1 clássica via `/legacy`, escolhida sobre a v2
  hooks-based por familiaridade/menor risco), drag/resize/reorder/esconder,
  persistido em `localStorage` por jogo (client-only, decisão deliberada).
  Validado em browser real via Playwright ad-hoc.
- Todas as 5 specs mergeadas na `main` (commits `--no-ff` por fatia),
  validadas ao vivo (grpcurl + curl + browser), 1062 testes Go verdes.

## Estado desde 2026-07-12

**`session/025-session-history` — FEITO, Fatias 1-4, 2026-07-13:** o histórico
de sessões fechadas do session-service agora é persistido em Postgres próprio
(`session-postgres` + exporter no compose), sobrevive a restart do serviço —
validado ao vivo (restart real, sessão fechada seguiu acessível). `Session`
ganhou `Participants []Participant{PlayerID, Role, JoinedAt}` (dedupe na
re-entrada; papel atualizado quando muda), acumulado em `Create`/`OpenWorld`/
`JoinWorld`. Dois endpoints novos no session-service (`GET .../sessions?
state=closed&limit=&before=` — mesma rota, mudou de fonte; `GET /admin/players
/{id}/sessions?gameId=`), paginados por cursor (`nextBefore`), com fallback
503 `history_unavailable` se o Postgres do histórico cair. admin-bff repassa +
enriquece com handle (specs admin-bff/023, admin-ui/061). admin-ui: aba
**History** nova no PlayerDetail (após Achievements) com papel/motivo/duração/
participantes por sessão; aba Sessions do jogo trocou a fonte das fechadas
para o store persistido com botão "Carregar mais". Critério 4 da spec (coop
morto aparece com `role=coop` no histórico) validado ao vivo em dois jogos.
Mergeado na `main` (branch `feat/session/025-session-history`, 4 commits).

## Estado desde 2026-07-08

**Fix do achado da bateria real do host_world_dropin + spec de histórico de sessão (2026-07-12):**

- **session-service — phantom morto sumia da sala do admin (follow-up #15 do arquétipo, FEITO):** o achado ao vivo "coop via sinais nunca aparece na sala" tinha causa real em `DIED`/`LEFT` apagando o jogador de `Players` (não no caminho dos sinais, que estava correto — repro limpa via grpcurl provou). `Players` de host_world agora acumula **participantes** (dedupe na re-entrada; rollback de token não vaza); `RoleSlots` seguem sendo a presença/capacidade. 176 testes verdes; validado ao vivo (DIED → slot libera, participante permanece).
- **`session/025-session-history` (spec Proposed, NADA implementado):** sessões são 100% in-memory (restart apaga — outro achado da bateria). A spec (spec/plan/tasks completos) cobre: `Participants{PlayerID, Role}` no domínio, Postgres próprio do session-service (write-once no close, best-effort), endpoints admin paginados (fechadas por jogo; sessões por player), aba **History** na página do player (após Achievements, escopo do jogo selecionado) e fechadas persistidas na aba Sessions do jogo. Implementar quando priorizado — item novo no backlog abaixo.

**Motor de regras de achievements — V2 completo (ADR-0027 + ADR-0028), 2026-07-09/10:**

- **Editor visual (canvas)**: React Flow, specs `admin-ui/043→053` — grafo de 4 camadas (facts → regras → event types → achievements), criação/edição de regra **in-canvas**, busca com destaque de caminho, view Tabela como alternativa. Kit reutilizável em `admin-ui/src/components/flow/`.
- **V2a — comparadores**: `MatchCond` aceita operadores (`lt/lte/gt/gte/ne/in`) além de igualdade — regras editáveis com limiar numérico (ex.: `placement lte 10`), sem hardcode. Stats do jogador entram como facts (`stat.<chave>`), habilitando condições compostas (ex.: *flawless victory* = `outcome=win` + `stat.deaths lte 0`).
- **V2b — supressão de regra global por jogo** (specs `achievements/017` + `admin-ui/051`): um jogo pode desligar, só pra si, uma regra global do arquétipo (tabela `event_rule_suppressions`, sem tocar a regra compartilhada) — a pergunta em aberto do ADR-0027. Visível no canvas (nó esmaecido, edges tracejadas).
- **V2c (prioridade/exclusividade)**: segue **gated por evidência** — não implementar sem caso real.
- **Polish**: specs `admin-ui/052` (ações do form no header, event type novo sempre visível) e `053` (Pipeline de dados movido de Analytics pra Configurações, mais coerente ali).

**Estresse-teste do motor — jogo "Achievements Test" (2026-07-10):** reaproveitado um jogo de teste (50 achievements genéricos de import CSV + 3 exemplos) como prova de conceito real: 2 modos registrados (`host_world_dropin` "Mundo Vivo" + `queue_arena` "Arena Ranqueada"), 66 achievements (a maioria convertida pra critério por **stat**, alguns reaproveitando regras globais por **evento**), 13 achievements novos cobrindo padrões famosos (Flawless Victory, Giant Slayer, Speedrunner etc.) via 6 regras compostas com comparadores. **Achou e fechou 2 lacunas reais do motor:**

- `achievements/018`: `host_world_dropin` era o único arquétipo sem suporte a stats no envelope `participation_event` — nenhum achievement por métrica nem regra com comparador funcionava nesse arquétipo até esta spec (espelha o que os outros 3 já tinham).
- `game-registry/011`: `HostWorldDropInParams` era o único perfil sem o campo `ScoringStats` — canvas sempre marcava stat de HWD como "drift" mesmo depois da 018. Fechado.
- Bônus: nit de data zero (`0001-01-01`) em achievements não desbloqueados corrigido (`admin-bff/021` — `omitempty` não omite `time.Time` não-ponteiro).

**Duas limitações genuínas do motor, documentadas e NÃO resolvidas** (fica pra decisão futura, sem plano de ação ainda): sem janela de tempo intra-partida (tipo "5 abates em 10s"); sem streak/consecutivo entre partidas ("vencer 5 seguidas"). Nenhuma tem caso real pedindo ainda.

**Analytics — Fase 1 de um plano em 3 fases (spec `admin-ui/054`, 2026-07-10):** a aba de Analytics **por jogo** tinha zero gráfico (só números + tabela condicional por arquétipo). Fase 1 acrescentou `LineChart`/`DonutChart`/`BarChart` (`@mantine/charts`, já instalado — avaliado e confirmado que não vale trocar de lib) usando dados que já vinham da API e nunca eram exibidos. Achado e fechado no caminho: `outcome` nunca ia pro DTO de participação (`analytics-service/008`) e a migration 006 desse serviço nunca tinha sido montada no `docker-compose.yml` (o endpoint de detalhe por jogo dava 500 pra qualquer jogo — bug pré-existente, não introduzido nesta sessão). **Fases 2/3 (série temporal pros outros 3 arquétipos + captura de stats brutos; dashboard configurável por widget) ficam gated por evidência.**

**ADRs novos desde 08/07:** 0027 (regras genéricas data-driven), 0028 (editor visual + roadmap do motor V2). **0026 (Fleet por jogo) já estava Accepted e implementado desde 2026-07-09/10** (Packet Arena e Capoeira Souls com fleet cadastrada) — este doc chegou a listar isso como pendente por desatualização, corrigido em 2026-07-11.

**Analytics — quebra por modo nas participações (spec `analytics-service/009`, 2026-07-11):** a aba Analytics por jogo agrupava participações só por arquétipo, não por modo nomeado (ADR-0024) — um jogo com vários modos host_world_dropin (Roda Aberta vs. Batizado do Capoeira Souls) não distinguia as métricas de cada um. Fechado ponta a ponta: `mode_id` na PK de `game_participation_rollup` + seletor de modo no admin-ui. Os outros 3 rollups (queue_arena/battle_royale/coop_mission) têm a mesma lacuna, documentada mas não implementada (schema mais invasivo — `PRIMARY KEY (game_id)` sem chave composta hoje).

**ADR-0029 (infra Agones centralizada) — Accepted e Fatia 1 implementada (2026-07-11):** bootstrap de cluster (k3s/k3d, Agones, kubeconfig, monitoramento Grafana) e Fleet manifests migraram de `unity-queue-arena-data-plane` para `go-multiplayer-control-plane/infra/agones/` — elimina a duplicação que o `host_world_dropin` teria na Fase 5. `apply-fleet.sh` agora é genérico (`<fleet-name>`, cobre k3d local e Pi 5). Fatia 2 (registry de imagem por jogo + CI/GHCR) segue gated por evidência. Pendente de você: rodar `infra/agones/pi5/pi5-tunnels.sh install` pra recarregar o LaunchAgent do túnel Pi apontando pro novo caminho (bloqueado pelo classificador de permissões do Claude Code por instalar um serviço persistente).

**Housekeeping:** ~14 commits locais à frente do `origin/main`, ainda não pushados. Confira `git log origin/main..HEAD` antes do próximo push.

---

## `host_world_dropin` — completo no control plane + console + contrato (2026-07-07)

Arquétipo Souls-like (ficha `docs/game-models/01-soulslike-invasion-coop.md`) — **quebra o modelo do `queue_arena` em quase todos os eixos** (sem partida formada por fila, sem placar terminal, sessão = mundo persistente de um host, papéis assimétricos, resultado por participante). **Fechado ponta a ponta no que é nosso**, na `main`, validado com server mockado. Roadmap: `docs/game-models/host_world_dropin-roadmap.md`.

| Fatia | Specs | O que entrega |
|---|---|---|
| **H0** — sessão host-cêntrica | `session/011` | `OpenWorld`/`JoinWorld`/`LeaveWorld`/`CloseWorld`; slots por papel (coop/invasor) do perfil; connect token com claim `role` |
| **H1a** — invasão (busca+injeção) | `session/012` + `matchmaking/006` | diretório de invadíveis (`ListInvadableWorlds`); RPC `Invade` filtra por faixa assimétrica (`Eligible` pura) e injeta `role=invader`; `UnaryAuthInterceptor` novo |
| **H1b** — co-op (quadro de sinais) | `matchmaking/007` | `PostSummonSign` (stream, aliado anuncia e aguarda), `ListSigns`, `SummonSign` (host invoca → injeta `role=coop`); store efêmero em memória + TTL; banda `CoopEligible` simétrica |
| **H2** — resultado por evento/papel | `session/013` + `achievements/009` + `analytics/004` | `ReportParticipationEvent` (JOINED/BOSS_DEFEATED/HOST_KILLED/DIED/LEFT); envelope `kind=participation_event` no mesmo tópico `match-results`; sinks despacham por `kind` (achievements por papel, analytics agrega participações) |
| **H3** — surfacing no console | `session/014` + `admin-bff/014` + `admin-ui/020` | board expõe `origin`/`host`/`roleSlots`; BFF repassa + endpoint per-game `/api/analytics/games/{id}`; Rooms mostra slots por papel (co-op X/2, invasor Y/1), GameAnalytics mostra participações por papel |
| **Contrato** | doc | `docs/game-models/host_world_dropin-dataplane-contract.md` — fonte de verdade control↔data plane (fluxos, claim `role`, `ReportParticipationEvent`, envelope, mudanças p/ o repo Unity) |

**Evoluções adiadas** (backlog com contexto para retomar): `docs/game-models/host_world_dropin-follow-ups.md` — senha/play-with-friends (✅ fechado, ver abaixo), pools/região, store Redis+pub/sub, alocação sob demanda, observabilidade fina do funil. Os itens **`eventId` por ocorrência**, **dimensão `outcome` no analytics** e **critério por stat na participação** já foram fechados (specs 007/018/008 acima). **Plano:** evoluir os detalhes restantes **conforme o data plane real for pedindo** — já está acontecendo.

**Cross-repo (Unity) — EM ANDAMENTO, não mais "falta criar":** `unity-host-world-dropin-data-plane` já existe (repo real no GitHub, 28 commits) e está nas **Fases 0–4 done** (mundo do host, phantoms join-in-progress, invasão+sinais via matchmaking incl. senha, combate/boss com ciclo de vida completo) — tudo verificado ao vivo, com bugs reais achados e corrigidos no processo (ex.: `Physics.SyncTransforms()` faltando antes de `OverlapSphere` de combate). Isso já validou o follow-up #1 (senha) na prática. Falta **Fase 5** (e2e orquestrado k3d→Pi 5 — ainda usa a Fleet Agones do `queue_arena`, não tem própria) e **Fase 6+** (refinamento). Detalhe completo: `unity-host-world-dropin-data-plane/docs/ROADMAP.md`. Leaderboard de invasões/co-ops é cauda opcional.

## Estado atual (`main` verde, stack local de pé)

Gameplay loop fechado e observável no painel:

`FindMatch(game_id) → match por jogo → CreateSession(game_id) → ReportResult → fan-out (achievements + leaderboard + analytics) → painel`

- **Multi-tenant por jogo** (ADR-0017) ponta a ponta: game-registry + achievements + leaderboard + player-data + cloud-save escopados por `game_id`.
- **Auth serviço-a-serviço** (ADR-0018): internal key (`X-Internal-Key`) nos control planes; `shared/auth` (`RequireInternalKey` + `InternalKeyTransport`). Chave dev: `dev-internal-key`.
- **Presença por jogo** (ADR-0019): identidade continua **global**; cada jogo tem uma player base explícita (`game_players` no game-registry), marcada no **login-into-game** (identity aceita `gameId` opcional, marca best-effort com timeout de 2s).
- **Fan-out durável** do resultado (ADR-0020, spec `session/006-durable-fanout`): `session` publica o resultado no **tópico SNS `match-results`**; achievements, leaderboard e analytics viram **consumidores SQS** (`*-consumer`) com retry, **DLQ** e **idempotência** (achievements/analytics via tabela `processed_*` + tx pgx; leaderboard idempotente de graça pelo `ZADD GT`). Local via **LocalStack** + **Terraform** (`infra/terraform/environments/local/messaging/`, provisionado no boot do compose). Validado ponta a ponta no Docker (publish → 3 filas → sinks refletem; reenvio = no-op).
- **Painel jogo-cêntrico** (admin-bff + admin-ui, Mantine) em http://localhost:8089 — ver seção abaixo.
- **Rooms Console** (Onda 1, specs `session/007` + `admin-bff/008` + `admin-ui/009`): board global das salas ao vivo (todos os jogos), enriquecido com handle + nome do jogo, polling 5s, **force-close** (`POST /admin/sessions/{matchId}/close`). Infrastructure tem os cards de **saúde do fan-out** (publish/consume via PromQL).
- **Matchmaking funnel** (`admin-ui/010`): fila/sessões/matches ok-falha no Infrastructure, polling 10s.
- **analytics-service** (ADR-0021) — assina `match-results` (fila SQS própria), agrega por **worker pool** em rollups por jogo idempotentes por `matchId` (Postgres). Aba **Analytics**: overview cross-game (LineChart) + **detalhe por jogo com gráficos** (spec `admin-ui/054`, 2026-07-10 — LineChart/DonutChart/BarChart por arquétipo).
- **Motor de achievements/event-rules** (ADR-0027/0028): data-driven, editor visual, comparadores, stats-as-facts, supressão de regra global por jogo — ver seção "Estado desde 2026-07-08" acima.
- **Rename de jogo**, **importação CSV de conquistas**, **remoção de jogo (soft delete)**, **janelas de leaderboard (daily/weekly/monthly) + paginação** — todos fechados (ver ADRs 0025 e specs correspondentes).
- **Observabilidade Grafana** — 7 dashboards provisionados (incluindo `game-lifecycle.json` e `analytics-workers.json`).

**Trilha de serviços da tese: 100% construída.** 11 serviços Go + 4 workers de fila (achievements/leaderboard/analytics-consumer): identity, game-registry, achievements, player-data, cloud-save, leaderboard, matchmaking (gRPC), session (gRPC), analytics, admin-bff, admin-ui. Observabilidade: Prometheus + Grafana (7 dashboards) + Tempo.

ADRs recentes: 0020 (fan-out durável), 0021 (analytics), 0022 (reconcile de órfãs), 0023 (perfil multiplayer), 0024 (modos + manifest), 0025 (remoção de jogo — soft delete), **0026 (alocação Agones fleet-aware por jogo — Proposed, não implementado)**, **0027 (regras genéricas data-driven)**, **0028 (editor visual + roadmap do motor V2)**.

> **Dois data planes Unity reais agora** (ver `docs/UNITY-INTEGRATION.md`, atualizado): `unity-queue-arena-data-plane` nas Fases 0–7 (Dedicated Server ARM64 via Agones, handshake completo `login → FindMatch → session aloca via Agones → connect token Ed25519 → partida → ReportResult` rodando de verdade) **e** `unity-host-world-dropin-data-plane` nas Fases 0–4 done (mundo do host, invasão/sinais incl. senha, combate com boss — tudo verificado ao vivo), Fase 5 pendente. `battle_royale`/`coop_mission` seguem sem repo Unity (ver backlog #1).

## ⚠️ Pendente de você (ações fora do código)

1. ~~**Push pro GitHub**~~ ✅ **FEITO (2026-07-08):** os dois repos publicados como **privados**. Desde então, commits locais seguem se acumulando sem novo push (ver "Housekeeping" no topo) — considerar sincronizar.
2. ~~**Rename do control plane**~~ ✅ **CONCLUÍDO (2026-07-08).**
3. ~~**ADR-0026**~~ ✅ **Accepted e implementado (2026-07-09/10)** — nada pendente.
4. **Reinstalar o LaunchAgent do túnel Pi 5** (`infra/agones/pi5/pi5-tunnels.sh install`) — a migração da ADR-0029 moveu o script; o LaunchAgent hoje ativo ainda aponta pro caminho antigo (removido). Ver seção ADR-0029 acima.

## Painel jogo-cêntrico (✅ completo)

Modelo final = **dois eixos** (identidade global + presença; ver ADR-0019). Não há mais "jogo ativo global" no header.

- **Games** (`admin-ui/005-game-library` + `003-generated-cover`): biblioteca de **capas 9:16** geradas no cliente por **DiceBear** a partir de `generatedImageSeed` (=slug). Abrir um jogo → abas **Achievements** (CRUD + import CSV + Event Rules com canvas visual), **Leaderboards**, **Players**, **Sessions**, **Analytics** (com gráficos), **Configurações** (dados do jogo + modos multiplayer + Pipeline de dados + zona de perigo).
- **Players**: base **global** do identity (paginada) + **avatares DiceBear** `adventurer`. Abrir um player → seletor escopado aos jogos DELE (consulta reversa da presença) → abas Achievements/Player data/Cloud saves.

> **Histórico por sessão:** vive no `git log` (Conventional Commits com escopo de serviço) e na tabela de estado de `docs/game-models/README.md`. Este handoff não é changelog — mantém só o estado atual + o backlog.

## Backlog priorizado (sem spec ainda)

Uma feature por branch `feat/<serviço>/<NNN-slug>`, merge `--no-ff` (`docs/GIT-WORKFLOW.md`). Itens concluídos desde a última revisão deste doc foram removidos da lista (ficam registrados no histórico de commits e nas seções acima); ordem sugerida do que resta:

1. ~~**Histórico de sessão**~~ ✅ **FEITO (2026-07-13)** — ver "Estado desde 2026-07-12" acima.
2. **Cross-repo Unity — em andamento, maior frente ativa.** `unity-queue-arena-data-plane` (Fases 0–7) e `unity-host-world-dropin-data-plane` (Fases 0–4 done, ver "Estado desde 2026-07-08" acima) têm data plane real; falta a **Fase 5** (e2e orquestrado k3d→Pi 5 + Fleet própria) do segundo — agora mais simples de bootstrapar graças à ADR-0029 (infra Agones centralizada) — e `battle_royale`/`coop_mission` ainda não têm repo (contrato pronto, zero código Unity).
3. **Infra / deploy — payoff da tese (ADR-0008).** Falta o grosso: demais serviços em Terraform, `environments/prod` (AWS real), compute por serviço (ADR-0005 — hoje só decidido "diferido"). É a medição de custo/performance vs AWS GameKit que a tese promete; ainda não começou.
4. **Service JWT** (evolução do ADR-0018): trocar a internal key compartilhada por tokens por serviço. Abrir ADR antes de implementar.
5. **SQS queue depth no Grafana** (observabilidade futura): precisa de `yet-another-cloudwatch-exporter` (YACE) — só faz sentido quando houver AWS real (item 2).
6. ~~**Analytics Fase 2/3**~~ ✅ **FEITO (2026-07-13)** — ver "Estado desde 2026-07-13" acima.
7. **V2c do motor de regras** (prioridade/exclusividade first-match-wins) — gated por evidência.
8. **As duas limitações do motor de achievements** achadas no estresse-teste (janela de tempo intra-partida; streak entre partidas) — sem plano de ação ainda, só documentadas.
9. **Fatia 2 da ADR-0029** (registry de imagem por jogo + CI/GHCR) — gated por evidência (2º arquétipo real em produção, ou outro dev precisando mexer sem acesso ao cluster).

## Gotchas operacionais

- **Migrations Postgres** só rodam em **volume novo**: ao mudar/adicionar migration, ou aplicar manualmente no volume em uso (`docker exec -i <postgres> psql -U dev -d <db> < migration.sql`) ou `docker compose ... rm -sfv <svc> <svc>-postgres` (apaga o volume anônimo) e remontar. Já mordeu várias vezes nesta sessão (achievements/010, analytics/006 — mount que faltava desde a spec 007 original).
- **Proto**: após mexer no module path / `.proto`, **regenerar com `buf`** (`PATH="$(go env GOPATH)/bin:$PATH" buf generate` dentro do serviço) — um `sed` no `.pb.go` corrompe o `rawDesc` (go_package tem prefixo de tamanho).
- **golangci-lint** não está instalado por padrão: `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2` (o CI usa a mesma versão pinada). Config em `.golangci.yml` (v2): errcheck dispensado só nos `_test.go`.
- **Prometheus** não raspa job novo sem `docker compose restart prometheus`; **Grafana** provisioning só no boot (restart do container).
- **Fan-out (LocalStack/Terraform):** os recursos SNS/SQS são criados pelo `messaging-provisioner` (Terraform) no boot — depende do LocalStack `healthy`. As envs `MATCH_RESULTS_TOPIC_ARN`/`MATCH_RESULTS_QUEUE_URL` no compose são **fixas e determinísticas**. Se mudar o `.tf`, o provisioner reaplica no próximo `up` (idempotente).
- **Processo órfão** segurando porta gRPC (50051/50052) de algum `go run` antigo: matar antes de `up`.
- **/metrics e /healthz** ficam **fora** do guard da internal key (Prometheus precisa raspar).
- Internal key dev: `dev-internal-key` (header `X-Internal-Key`).
- **admin-ui bundle**: DiceBear `adventurer` é pesado (~280kB); já mitigado com code-splitting. `@mantine/charts`/`recharts` (LineChart) também entram em chunk lazy próprio.
- **Canvas do editor de regras** (`rulesGraph.ts`): alturas de nó são ESTIMADAS no layout — se um card mudar de conteúdo/CSS, recalibrar medindo `offsetHeight` real via Playwright (gotcha batido várias vezes nas specs 049/050/052).
