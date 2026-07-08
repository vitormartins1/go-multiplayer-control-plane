# NEXT-STEPS — handoff (atualizado 2026-07-08)

Ponto de retomada após limpar o contexto. Resume **o estado atual**, **o que fica pendente de você**, **o que foi feito nesta sessão** e o **backlog priorizado**.

> **ADR-0023 — COMPLETO no control plane (2026-07-07): os 4 arquétipos ponta a ponta.** Estado por fatia: `docs/game-models/README.md` → "Estado da implementação". Todos validados com server mockado (adapter `static` + cliente de teste); nenhum data plane Unity específico existe ainda além do `queue_arena`:
>
> - **`queue_arena`** (FFA/NvN, `win_loss_draw`) — completo + contrato; data plane de referência existe (`go-game-dataplane`, Fases 0–7).
> - **`host_world_dropin`** (Souls-like, `participation_event`) — H0–H3 + contrato; follow-ups em `host_world_dropin-follow-ups.md`.
> - **`battle_royale`** (lobby/squads, `placement`) — BR0–BR4 + contrato.
> - **`coop_mission`** (co-op vs. ambiente, `collective`) — C0–C4 + contrato (C4 mergeado na main em 2026-07-07).
>
> **Frente ativa nova: ADR-0024 — modos de jogo (perfis nomeados) + session manifest** (Accepted, 2026-07-07; origem no review do ADR-0023). Jogo 1—N modos (`modeId`), snapshot do perfil por sessão, validação `kind`×arquétipo no `ReportResult`, manifest p/ o data plane ler params (mata o `first-to-5` hardcoded), `modeId` aditivo no envelope. Fatias **M0–M2** em `docs/game-models/multiplayer-modes-roadmap.md`; **M0 e M1 COMPLETOS ✅ na main** (2026-07-07): M0 = coleção de modos no registry + console; M1 = `mode_id` ponta a ponta (fila por (game, mode), **snapshot imutável do perfil por sessão**, claim `mode_id` no token, **RPC `GetSessionConfig`/manifest** — mata config hardcoded no data plane —, validação kind×arquétipo no `ReportResult`, `modeId` nos envelopes) + os 4 contratos de data plane atualizados. **M2 (segmentação de sinks por modo) segue GATED por evidência** — não implementar sem demanda real. **Próximo natural: cross-repo Unity** — `unity-host-world-dropin-data-plane` (prioridade do dono) consumindo contrato + manifest; e o `go-game-dataplane` (queue_arena) tem migração pendente (first-to-5 → manifest; ver contrato).
>
> Docs de suporte: **`docs/REPO-TOPOLOGY.md`** — rename do control plane **executado** (module path `go-multiplayer-control-plane`; falta o `mv` da pasta); SDK Unity compartilhado registrado como **futuro** (princípio 5). Princípio firmado: **forma certa do control plane acima de compatibilidade do data plane**.

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

**Evoluções adiadas** (backlog com contexto para retomar): `docs/game-models/host_world_dropin-follow-ups.md` — senha/play-with-friends, pools/região, store Redis+pub/sub, `eventId` por ocorrência, dimensão `outcome` no analytics, critério por stat, alocação sob demanda, observabilidade fina do funil. **Plano:** evoluir esses detalhes **depois de validar a funcionalidade no data plane específico** (Unity).

**Falta (cross-repo, não control plane):** criar o `unity-host-world-dropin-data-plane` (Unity, ⬜ planejado em `REPO-TOPOLOGY.md`) consumindo o contrato + Agones — o e2e real do drop-in. Leaderboard de invasões/co-ops é cauda opcional.

## Estado atual (`main` verde, stack local de pé)

Gameplay loop fechado e observável no painel:

`FindMatch(game_id) → match por jogo → CreateSession(game_id) → ReportResult → fan-out (achievements + leaderboard) → painel`

- **Multi-tenant por jogo** (ADR-0017) ponta a ponta: game-registry + achievements + leaderboard + player-data + cloud-save escopados por `game_id`.
- **Auth serviço-a-serviço** (ADR-0018): internal key (`X-Internal-Key`) nos control planes; `shared/auth` (`RequireInternalKey` + `InternalKeyTransport`). Chave dev: `dev-internal-key`.
- **Presença por jogo** (ADR-0019): identidade continua **global**; cada jogo tem uma player base explícita (`game_players` no game-registry), marcada no **login-into-game** (identity aceita `gameId` opcional, marca best-effort com timeout de 2s).
- **Fan-out durável** do resultado (ADR-0020, spec `session/006-durable-fanout`): `session` publica o resultado no **tópico SNS `match-results`**; achievements e leaderboard viram **consumidores SQS** (`*-consumer`) com retry, **DLQ** e **idempotência por `matchID`** (achievements via tabela `processed_matches` + tx pgx; leaderboard idempotente de graça pelo `ZADD GT`). Local via **LocalStack** + **Terraform** (`infra/terraform/environments/local/messaging/`, provisionado no boot do compose). Substitui o `CompositePublisher` HTTP best-effort das specs 002/003 (**removido**). Validado ponta a ponta no Docker (publish → 2 filas → sinks refletem; reenvio = no-op).
- **Painel jogo-cêntrico** (admin-bff + admin-ui, Mantine) em http://localhost:8089 — ver seção abaixo.
- **Rooms Console** (Onda 1, specs `session/007` + `admin-bff/008` + `admin-ui/009`): o painel deixou de ser só leitura. Aba **Rooms** = board global das salas ao vivo (todos os jogos, enriquecido com handle + nome do jogo, polling 5s) com **force-close** (`POST /admin/sessions/{matchId}/close`, motivo `admin`, libera o server). Infrastructure ganhou os cards de **saúde do fan-out** (publish/consume via PromQL). Validado com a stack completa.
- **Matchmaking funnel** (`admin-ui/010`): liga o `GET /api/metrics/live` (já existente) num bloco de cards no Infrastructure (fila/sessões/matches ok/falha), polling 10s. Fecha o ciclo de observação: fila → match → Rooms → fan-out.
- **analytics-service** (ADR-0021, specs `analytics/001` + `admin-bff/009` + `admin-ui/011`) — **trilha de serviços completa**. Assina o tópico `match-results` (nova fila SQS, zero mudança no produtor), agrega por um **worker pool** (goroutines/channels/context; GO-LEARNINGS #18) em rollups por jogo idempotentes por `matchId` (Postgres). Acende a aba **Analytics** (overview cross-game). Validado ponta a ponta (agregação + idempotência).
- **Refinos do console (item 3)** — **rename de jogo** (specs `game-registry/005` + `admin-bff/010` + `admin-ui/012`: aba Configurações renomeia, `PUT /games/{id}` só o nome) e **gráfico "partidas por dia"** na Analytics (specs `analytics/002` + `admin-ui/013`: série `daily` cross-game + `LineChart` do `@mantine/charts`, chunk lazy). **Delete de jogo continua adiado** (ADR da cascata — ver backlog).
- **Importação CSV de conquistas** (specs `admin-bff/011` + `admin-ui/014`): botão "Baixar modelo" (template CSV client-side) e "Importar CSV" na aba Achievements do jogo. Parser CSV nativo no frontend (`lib/csv.ts`), validação pré-envio por linha, modal de erros de validação e modal de resultado pós-importação. BFF: `POST /api/games/{gameId}/achievements/import` (best-effort, resultado por linha). Testes unitários completos (client + handler). Commit `1947ce4`.
- **Observabilidade Grafana** — review completo + 2 novos dashboards provisionados: `game-lifecycle.json` (funil matchmaking → sessões → fan-out consumers) e `analytics-workers.json` (worker pool + backpressure). Commit `1947ce4`.

**Trilha de serviços da tese: 100% construída.** 11 serviços Go + 3 workers de fila (achievements/leaderboard/analytics-consumer): identity, game-registry, achievements, player-data, cloud-save, leaderboard, matchmaking (gRPC), session (gRPC), **analytics**, admin-bff, admin-ui. Observabilidade: Prometheus + Grafana (7 dashboards) + Tempo.

ADRs recentes: 0018 (service auth), 0019 (presença por jogo), 0020 (fan-out durável SNS→SQS), 0021 (analytics via worker pool), 0022 (reconcile de órfãs), 0023 (perfil multiplayer), 0024 (modos + manifest), **0025 (remoção de jogo — soft delete)**.

**Fechados em 2026-07-08 (pós-ADR-0024):**

- **Remoção de jogo** (ADR-0025, specs `game-registry/010` + `admin-bff/018` + `admin-ui/038`): soft delete no registry como ponto de corte único (`deleted_at`, filtro na fonte no repository — jogo deletado = inexistente por fora); presença recusada, reversa omite, slug reservado; zona de perigo no console com confirmação pelo nome. Purge assíncrono = evolução gated (Opção C do ADR).
- **Leaderboard: janelas por período + paginação** (specs `leaderboard/007` + `admin-bff/019` + `admin-ui/039`): rankings `daily`/`weekly`/`monthly` derivados do relógio UTC sobre a key Redis existente (all-time intacta, zero migration); writes espalham nas 4 janelas (idempotência única no cumulativo; consumer intocado); `?window`/`?offset` ponta a ponta até a aba Leaderboards (SegmentedControl + "Carregar mais"). **Seasons nomeadas adiadas por evidência** (exigirão ADR sobre onde vive a config).

> **Data plane muito à frente do que este doc registrava:** o `go-game-dataplane` (Unity) fechou as Fases 0–6 e está na 7 — Dedicated Server ARM64 orquestrado por Agones (k3d local → homelab Pi 5), com o handshake completo `login → FindMatch → session aloca via Agones → connect token Ed25519 → partida → ReportResult` rodando de verdade. Foi isso que tornou o fan-out durável urgente (game server real reportando resultado). `docs/UNITY-INTEGRATION.md` está **desatualizado** (marcado "rascunho" com TODOs já resolvidos) — candidato a atualização.

## ⚠️ Pendente de você (ações fora do código)

1. **Push pro GitHub — adiado por decisão (2026-07-07):** o repo fica **local** por ora. Quando for publicar, criar já como `github.com/vitormartins1/go-multiplayer-control-plane` (o module path já é esse), `git remote add origin …`, `git push -u origin main`. Só então o CI (GitHub Actions) roda de verdade.
2. **Rename do control plane — falta só o `mv` da pasta (instruções detalhadas dadas em 2026-07-08):** module path Go + protos (`buf`) + docs já usam `go-multiplayer-control-plane` desde 2026-07-07; a varredura de 2026-07-08 confirmou que só restam menções históricas intencionais. Roteiro: (a) fechar sessões/IDE e **parar** o compose com `docker compose -f infra/docker/docker-compose.yml stop` (**não** `down`: os Postgres usam volumes anônimos — `down` remove os containers e o próximo `up` criaria volumes novos, perdendo os dados de demo; o project name do compose deriva da pasta `infra/docker`, não do nome do repo, então o rename não o afeta); (b) `cd ~/projects && mv go-game-backend go-multiplayer-control-plane`; (c) mover a memória do assistente: `mv ~/.claude/projects/-Users-vitormartins-projects-go-game-backend ~/.claude/projects/-Users-vitormartins-projects-go-multiplayer-control-plane`; (d) reabrir na pasta nova, `go build ./...` + `docker compose -f infra/docker/docker-compose.yml up -d` — **`up`, não `start`**: os bind mounts (prometheus.yml, grafana/provisioning, migrations) gravam paths absolutos nos containers, então eles precisam ser **recriados** apontando para a pasta nova; a recriação do compose **reaproveita os volumes anônimos** por padrão (dados de demo preservados — é o `down` que os órfãos). Data planes: `unity-<archetype>-data-plane` ao criar cada um; `go-game-dataplane` renomeia quando for tocado (`REPO-TOPOLOGY.md`).

## Painel jogo-cêntrico (✅ completo)

Modelo final = **dois eixos** (identidade global + presença; ver ADR-0019). Não há mais "jogo ativo global" no header.

- **Games** (`admin-ui/005-game-library` + `003-generated-cover`): biblioteca de **capas 9:16** geradas no cliente por **DiceBear** a partir de `generatedImageSeed` (=slug); metadados no game-registry (`image`/`imageSource`/`generatedImageSeed`/`generatedImageStyle`) deixam espaço p/ upload futuro. Abrir um jogo → abas **Achievements** (CRUD completo, `achievements/006-catalog-crud`), **Leaderboards**, **Players** (base do jogo = presença), Configurações (ComingSoon).
- **Players**: base **global** do identity (paginada) + **avatares DiceBear** `adventurer` (seed = playerId, `admin-ui/006-player-avatars`). Abrir um player → **seletor escopado aos jogos DELE** (`GET /api/players/{id}/games`, consulta reversa da presença, `game-registry/004-games-of-player`) → abas Achievements/Player data/Cloud saves.

> **Histórico por sessão:** vive no `git log` (Conventional Commits com escopo de serviço) e na tabela de estado de `docs/game-models/README.md`. Este handoff não é changelog — mantém só o estado atual + o backlog. O trabalho mais recente é o arco `host_world_dropin` (bloco acima).

## Backlog priorizado (sem spec ainda)

Uma feature por branch `feat/<serviço>/<NNN-slug>`, merge `--no-ff` (`docs/GIT-WORKFLOW.md`). Ordem sugerida:

1. **Push pro GitHub** (acima) → CI passa a rodar. **Atenção:** o CI agora precisa de Docker p/ os testes de integração/e2e do fan-out (LocalStack + Postgres via testcontainers).
2. ✅ **Fan-out resiliente — FEITO** (ADR-0020, spec `session/006-durable-fanout`).
3. ✅ **analytics-service — FEITO** (ADR-0021). *Evoluções v1→v2 quando fizer sentido:* telemetria genérica (`POST /events`); deep-dive por jogo (aba no GameDetail). O **event store cru** (C2 do ADR-0021) é o escape se a pré-agregação engessar.
4. ✅ **Importação CSV de conquistas — FEITA** (specs `admin-bff/011` + `admin-ui/014`, commit `1947ce4`).
5. ✅ **Remover jogo — FEITO (2026-07-08)**: ADR-0025 (soft delete no registry; hard purge = evolução gated por evidência) + specs `game-registry/010` (deleted_at, filtro na fonte no repository, DELETE /games/{id}) + `admin-bff/018` (passthrough) + `admin-ui/038` (zona de perigo + modal digitando o nome). Validado e2e no compose (204/404, some da lista, slug reservado 409).
6. ✅ **Leaderboard: janelas por período + paginação — FEITO (2026-07-08)** (specs `leaderboard/007` + `admin-bff/019` + `admin-ui/039`): rankings daily/weekly/monthly derivados do relógio UTC (zero config) sobre a key Redis existente (all-time intacta); writes espalham nas 4 janelas (guarda de idempotência única no cumulativo); `?window`/`?offset` ponta a ponta até o console (SegmentedControl + "Carregar mais"). **Seasons nomeadas** (rollover manual/config) seguem **adiadas por evidência** — exigirão ADR (onde vive a config).
7. ✅ **Cleanup admin-bff — FEITO**: extraído `internal/upstream` (`Do`, `ErrorCode`, `DefaultClient`) — 5 adapters (achievements, cloudsave, playerdata, leaderboard, gameregistry) sem mais `do()`/`classify()`/`errorCode()` locais.
8. **SQS queue depth no Grafana** (observabilidade futura): visibilidade de profundidade de fila e DLQ requer `yet-another-cloudwatch-exporter` (YACE). Métricas do lado consumidor já estão no dashboard `game-lifecycle`. Implementar quando chegar em produção AWS.
9. **Service JWT** (evolução do ADR-0018): trocar a internal key compartilhada por tokens por serviço. Abrir ADR.
10. **Infra / deploy — payoff da tese (ADR-0008):** falta o grosso: demais serviços em Terraform, `environments/prod` (AWS real), compute por serviço (ADR-0005); medir custo/performance vs AWS GameKit.
11. **Data plane / o jogo (repos Unity por arquétipo, não-Go — `REPO-TOPOLOGY.md`):** o `go-game-dataplane` (queue_arena, Fases 0–7) valida o `queue_arena`; o **`unity-host-world-dropin-data-plane`** (⬜ planejado) é o próximo — encarna o mundo do host consumindo `host_world_dropin-dataplane-contract.md` (claim `role`, join-in-progress, `ReportParticipationEvent`) + Agones. É onde a funcionalidade do drop-in vira e2e real e destrava os follow-ups do arquétipo.
12. ✅ **Sessão órfã: fechar por sinal do GameServer — FEITO (opção a)** (ADR-0022, spec `session/008-orphan-reconcile`). O session-service **reconcilia** as sessões OPEN contra o Agones a cada tick do sweeper: nova Port `domain.ServerRegistry.LiveServerIDs` (LIST dos GameServers da fleet por label — só o adapter Agones implementa, `nil` no modo static) fecha com motivo novo **`abandoned`** toda sessão cujo `ServerID` sumiu da fleet, respeitando um **grace period** (`SESSION_ORPHAN_GRACE`, default 60s, contra staleness do LIST). Falha no LIST = tick pulado (não fecha nada). O TTL sweeper vira **backstop** (e único mecanismo no static). Métrica `sessions_closed_total{reason="abandoned"}` distinta; o painel "Sessões encerradas/s por motivo" pega o motivo sozinho (`{{reason}}`). GO-LEARNINGS #19 (dependência opcional como interface nil + armadilha do typed-nil). *Complemento futuro adiado:* opção (b) — game server **notificar abandono** no `Shutdown` — não cobre crash e é cross-repo, fica como evolução. *Não era bug de métrica:* `session_active_sessions` = `len(s.sessions)` e o `closeSession` faz `delete` — decrementa certo; o "6 na métrica × 0 na lista" pós-limpeza foi só lag de scrape do Prometheus, não contabilidade furada.

## Gotchas operacionais

- **Migrations Postgres** só rodam em **volume novo**: ao mudar/adicionar migration, `docker compose ... rm -sfv <svc> <svc>-postgres` (apaga o volume anônimo) e remontar a migration no volume do compose. `game-registry-postgres` já teve 3 recriações (migrations 003/004).
- **Proto**: após mexer no module path / `.proto`, **regenerar com `buf`** (`PATH="$(go env GOPATH)/bin:$PATH" buf generate` dentro do serviço) — um `sed` no `.pb.go` corrompe o `rawDesc` (go_package tem prefixo de tamanho).
- **golangci-lint** não está instalado por padrão: `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2` (o CI usa a mesma versão pinada). Config em `.golangci.yml` (v2): errcheck dispensado só nos `_test.go`.
- **Prometheus** não raspa job novo sem `docker compose restart prometheus`; **Grafana** provisioning só no boot (restart do container).
- **Fan-out (LocalStack/Terraform):** os recursos SNS/SQS são criados pelo `messaging-provisioner` (Terraform) no boot — depende do LocalStack `healthy`. As envs `MATCH_RESULTS_TOPIC_ARN`/`MATCH_RESULTS_QUEUE_URL` no compose são **fixas e determinísticas** (conta `000000000000`, host `localstack:4566` no formato path-based — funciona mesmo o LocalStack devolvendo a URL `sqs.us-east-1.localhost.localstack.cloud`). Se mudar o `.tf`, o provisioner reaplica no próximo `up` (idempotente). A migration `004_processed_matches` exige recriar o volume do `achievements-postgres` (gotcha de migrations acima).
- **Processo órfão** segurando porta gRPC (50051/50052) de algum `go run` antigo: matar antes de `up`.
- **/metrics e /healthz** ficam **fora** do guard da internal key (Prometheus precisa raspar).
- Internal key dev: `dev-internal-key` (header `X-Internal-Key`).
- **admin-ui bundle**: DiceBear `adventurer` é pesado (~280kB); já mitigado com code-splitting (só carrega em Games/Players).
