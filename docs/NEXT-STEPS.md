# NEXT-STEPS — handoff (atualizado 2026-07-10)

Ponto de retomada após limpar o contexto. Resume **o estado atual**, **o que fica pendente de você**, **o que foi feito nesta sessão** e o **backlog priorizado**.

> **Trilha de serviços do control plane: 100% construída e validada — os 4 arquétipos ponta a ponta, motor de achievements/event-rules maduro, Analytics com gráficos.** O que falta de verdade agora é **fora deste repo** (data planes Unity) ou **infra real** (AWS). Ver "Estado desde 2026-07-08" logo abaixo pro que mudou nas últimas sessões.

## Estado desde 2026-07-08 (o que este doc ainda não registrava)

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

**ADRs novos desde 08/07:** 0027 (regras genéricas data-driven), 0028 (editor visual + roadmap do motor V2). Há também o **0026 (alocação Agones fleet-aware por jogo) em status Proposed** — não implementado, sem specs associadas ainda; decisão pendente de retomada.

**Housekeeping:** ~10 commits locais à frente do `origin/main`, ainda não pushados (specs 017/051 até 054, branches `feat/*` merged `--no-ff` na main local). Confira `git log origin/main..HEAD` antes do próximo push.

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

**Evoluções adiadas** (backlog com contexto para retomar): `docs/game-models/host_world_dropin-follow-ups.md` — senha/play-with-friends, pools/região, store Redis+pub/sub, alocação sob demanda, observabilidade fina do funil. Os itens **`eventId` por ocorrência** e **dimensão `outcome` no analytics** já foram fechados (specs 007/018/008 acima). **Plano:** evoluir os detalhes restantes **depois de validar a funcionalidade no data plane específico** (Unity).

**Falta (cross-repo, não control plane):** criar o `unity-host-world-dropin-data-plane` (Unity, ⬜ planejado em `REPO-TOPOLOGY.md`) consumindo o contrato + Agones — o e2e real do drop-in. Leaderboard de invasões/co-ops é cauda opcional.

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

> **Data plane muito à frente do que este doc registrava antes:** o `go-game-dataplane` (Unity) fechou as Fases 0–6 e está na 7 — Dedicated Server ARM64 orquestrado por Agones (k3d local → homelab Pi 5), com o handshake completo `login → FindMatch → session aloca via Agones → connect token Ed25519 → partida → ReportResult` rodando de verdade. Só cobre `queue_arena` — os outros 3 arquétipos não têm data plane Unity ainda (ver backlog #11). `docs/UNITY-INTEGRATION.md` está **desatualizado** — candidato a atualização.

## ⚠️ Pendente de você (ações fora do código)

1. ~~**Push pro GitHub**~~ ✅ **FEITO (2026-07-08):** os dois repos publicados como **privados**. Desde então, ~10 commits locais se acumularam sem novo push (ver "Housekeeping" no topo) — considerar sincronizar.
2. ~~**Rename do control plane**~~ ✅ **CONCLUÍDO (2026-07-08).**
3. **ADR-0026 (Agones fleet-aware por jogo)** está em status **Proposed** desde 2026-07-09, sem specs associadas — decidir se aceita/descarta/revisita quando for mexer em alocação Agones de novo.

## Painel jogo-cêntrico (✅ completo)

Modelo final = **dois eixos** (identidade global + presença; ver ADR-0019). Não há mais "jogo ativo global" no header.

- **Games** (`admin-ui/005-game-library` + `003-generated-cover`): biblioteca de **capas 9:16** geradas no cliente por **DiceBear** a partir de `generatedImageSeed` (=slug). Abrir um jogo → abas **Achievements** (CRUD + import CSV + Event Rules com canvas visual), **Leaderboards**, **Players**, **Sessions**, **Analytics** (com gráficos), **Configurações** (dados do jogo + modos multiplayer + Pipeline de dados + zona de perigo).
- **Players**: base **global** do identity (paginada) + **avatares DiceBear** `adventurer`. Abrir um player → seletor escopado aos jogos DELE (consulta reversa da presença) → abas Achievements/Player data/Cloud saves.

> **Histórico por sessão:** vive no `git log` (Conventional Commits com escopo de serviço) e na tabela de estado de `docs/game-models/README.md`. Este handoff não é changelog — mantém só o estado atual + o backlog.

## Backlog priorizado (sem spec ainda)

Uma feature por branch `feat/<serviço>/<NNN-slug>`, merge `--no-ff` (`docs/GIT-WORKFLOW.md`). Itens concluídos desde a última revisão deste doc foram removidos da lista (ficam registrados no histórico de commits e nas seções acima); ordem sugerida do que resta:

1. **Cross-repo Unity — o maior "falta fazer" real.** `go-game-dataplane` (queue_arena, Fases 0–7) é o único arquétipo com data plane de verdade. Os outros 3 têm contrato pronto (`docs/game-models/*-dataplane-contract.md`) mas zero código Unity: **`unity-host-world-dropin-data-plane`** é o próximo natural (prioridade histórica do dono) — encarna o mundo do host consumindo o contrato + Agones. Sem isso, os follow-ups adiados de cada arquétipo (`*-follow-ups.md`) não têm como ser validados com evidência real.
2. **Infra / deploy — payoff da tese (ADR-0008).** Falta o grosso: demais serviços em Terraform, `environments/prod` (AWS real), compute por serviço (ADR-0005 — hoje só decidido "diferido"). É a medição de custo/performance vs AWS GameKit que a tese promete; ainda não começou.
3. **Service JWT** (evolução do ADR-0018): trocar a internal key compartilhada por tokens por serviço. Abrir ADR antes de implementar.
4. **SQS queue depth no Grafana** (observabilidade futura): precisa de `yet-another-cloudwatch-exporter` (YACE) — só faz sentido quando houver AWS real (item 2).
5. **ADR-0026 (Agones fleet-aware por jogo)** — Proposed, sem specs. Decidir se retoma.
6. **Analytics Fase 2/3** (gated por evidência): série temporal pros outros 3 arquétipos (schema novo); captura de `stats` brutos (kills/deaths) no analytics-service; dashboard configurável por widget.
7. **V2c do motor de regras** (prioridade/exclusividade first-match-wins) — gated por evidência.
8. **As duas limitações do motor de achievements** achadas no estresse-teste (janela de tempo intra-partida; streak entre partidas) — sem plano de ação ainda, só documentadas.

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
