# NEXT-STEPS — handoff (atualizado 2026-07-07)

Ponto de retomada após limpar o contexto. Resume **o estado atual**, **o que fica pendente de você**, **o que foi feito nesta sessão** e o **backlog priorizado**.

> **Frente ativa (2026-07-07): ADR-0023 — multiplayer por arquétipo/perfil.** O estado por fatia vive em `docs/game-models/README.md` → "Estado da implementação". **Dois arquétipos completos no control plane** (validados com server mockado — adapter `static` + cliente de teste):
>
> - **`queue_arena`** (partida simétrica FFA/NvN, resultado win/loss/draw): declarar → formar (com **composição de times**, `team_id` no connect token) → resultado terminal → fan-out → console. Contrato: `docs/game-models/queue_arena-dataplane-contract.md`.
> - **`host_world_dropin`** (Souls-like — mundo persistente de um host, phantoms co-op/invasor por papel, resultado por evento): **H0–H3 + contrato FEITOS**. Ver o bloco dedicado abaixo.
>
> Docs de suporte: **`docs/REPO-TOPOLOGY.md`** (um `go-game-<archetype>-dataplane` por arquétipo + rename proposto do backend → `go-game-control-plane`; **decisão pendente**, nada renomeado). Princípio firmado: **forma certa do control plane acima de compatibilidade do data plane** — pode quebrar o Unity, migra-se depois.

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

**Falta (cross-repo, não control plane):** criar o `go-game-host-world-dropin-dataplane` (Unity, ⬜ planejado em `REPO-TOPOLOGY.md`) consumindo o contrato + Agones — o e2e real do drop-in. Leaderboard de invasões/co-ops é cauda opcional.

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

ADRs recentes: 0018 (service auth), 0019 (presença por jogo), 0020 (fan-out durável SNS→SQS), **0021 (analytics via worker pool)**.

> **Data plane muito à frente do que este doc registrava:** o `go-game-dataplane` (Unity) fechou as Fases 0–6 e está na 7 — Dedicated Server ARM64 orquestrado por Agones (k3d local → homelab Pi 5), com o handshake completo `login → FindMatch → session aloca via Agones → connect token Ed25519 → partida → ReportResult` rodando de verdade. Foi isso que tornou o fan-out durável urgente (game server real reportando resultado). `docs/UNITY-INTEGRATION.md` está **desatualizado** (marcado "rascunho" com TODOs já resolvidos) — candidato a atualização.

## ⚠️ Pendente de você (ações fora do código)

1. **Push pro GitHub** — o repo ainda é **local**. Criar `github.com/vitormartins1/go-game-backend`, `git remote add origin …`, `git push -u origin main`. **Só então o CI (GitHub Actions) roda de verdade.** Reproduzi os 4 jobs verdes localmente, mas o ambiente do Actions nunca rodou (testcontainers às vezes pede ajuste no runner).
2. **Nomes de repo decididos:** `go-game-backend` (este) e `go-game-unity` (o data plane Unity, repo separado — ADR-0010). O module path já foi renomeado de `c130n/game-backend` → **`github.com/vitormartins1/go-game-backend`**.

## Painel jogo-cêntrico (✅ completo)

Modelo final = **dois eixos** (identidade global + presença; ver ADR-0019). Não há mais "jogo ativo global" no header.

- **Games** (`admin-ui/005-game-library` + `003-generated-cover`): biblioteca de **capas 9:16** geradas no cliente por **DiceBear** a partir de `generatedImageSeed` (=slug); metadados no game-registry (`image`/`imageSource`/`generatedImageSeed`/`generatedImageStyle`) deixam espaço p/ upload futuro. Abrir um jogo → abas **Achievements** (CRUD completo, `achievements/006-catalog-crud`), **Leaderboards**, **Players** (base do jogo = presença), Configurações (ComingSoon).
- **Players**: base **global** do identity (paginada) + **avatares DiceBear** `adventurer` (seed = playerId, `admin-ui/006-player-avatars`). Abrir um player → **seletor escopado aos jogos DELE** (`GET /api/players/{id}/games`, consulta reversa da presença, `game-registry/004-games-of-player`) → abas Achievements/Player data/Cloud saves.

## O que foi feito nesta sessão (para referência)

**Admin-UI — expansão do painel (sem specs formais — usaram endpoints já existentes):**

- **Analytics por jogo** (`GameAnalytics.tsx`): aba Analytics no GameDetail; filtra o `analyticsOverview` cross-game pelo `gameId` e exibe 4 StatCards (partidas, jogadores únicos, vitórias, score médio).
- **JSON viewer** (Player Data e Cloud Save): valores objeto/array renderizados como `<Code block>` com scroll; primitivos inline — facilita inspeção de dados de gameplay.
- **Busca de jogadores** (`Players.tsx`): `TextInput` filtra a lista local por handle; paginação oculta durante busca ativa.
- **Ajuste de score no leaderboard** (feature 8): novo endpoint `PUT /admin/games/{g}/players/{p}/scores` no **leaderboard-service** (Redis `ZADD` sem GT — admin pode diminuir) + passthrough no **admin-bff** + modal com `NumberInput` na UI. Distingue de `adminSubmit` (highscore via `ZADD GT`).
- **Histórico de progresso de conquista** (feature 7): novo endpoint `GET /admin/games/{g}/achievements/{id}/progress` no **achievements-service** (query `player_achievements ORDER BY progress DESC`) + passthrough no **admin-bff** + modal na aba Achievements mostrando todos os jogadores com progress, status (desbloqueado/em progresso) e data.
- **CountBadge** (badges de contagem nas abas): suporta até 3 caracteres; cap em 999 (não "99+"); removido o prop `circle` do Mantine que espremía o conteúdo.

**Admin-bff cleanup:**

- Extraído `internal/upstream` (`DefaultClient`, `ErrorCode`, `Do`) — lógica de classificação 2xx/4xx/5xx centralizada; 5 adapters (achievements, cloudsave, playerdata, leaderboard, gameregistry) sem mais `do()`/`classify()`/`errorCode()` locais. Build + testes com `-race` verdes.

*Sessões anteriores (histórico):*

- Commit `1947ce4`: Review Grafana, dashboards `game-lifecycle.json` + `analytics-workers.json`, importação CSV de conquistas (specs `admin-bff/011` + `admin-ui/014`).
- `player-data/002-game-scope`, `cloud-save/002-game-scope`, `admin-bff/006-player-tabs`, `admin-ui/005-game-library`, `game-registry/002-004`, `achievements/006-catalog-crud`, chore module path + CI + code-splitting, code-review `high`.

## Backlog priorizado (sem spec ainda)

Uma feature por branch `feat/<serviço>/<NNN-slug>`, merge `--no-ff` (`docs/GIT-WORKFLOW.md`). Ordem sugerida:

1. **Push pro GitHub** (acima) → CI passa a rodar. **Atenção:** o CI agora precisa de Docker p/ os testes de integração/e2e do fan-out (LocalStack + Postgres via testcontainers).
2. ✅ **Fan-out resiliente — FEITO** (ADR-0020, spec `session/006-durable-fanout`).
3. ✅ **analytics-service — FEITO** (ADR-0021). *Evoluções v1→v2 quando fizer sentido:* telemetria genérica (`POST /events`); deep-dive por jogo (aba no GameDetail). O **event store cru** (C2 do ADR-0021) é o escape se a pré-agregação engessar.
4. ✅ **Importação CSV de conquistas — FEITA** (specs `admin-bff/011` + `admin-ui/014`, commit `1947ce4`).
5. **Remover jogo** no painel — *rename* ✅ feito. *Delete* segue **adiado** por precisar de decisão sobre dado órfão (cascata cross-service). **Precisa de ADR** — opções: *soft delete* (mais barato) vs *hard cascade*. A aba Configurações já avisa "remover ainda não disponível".
6. **Leaderboard**: seasons/janelas por período + paginação.
7. ✅ **Cleanup admin-bff — FEITO**: extraído `internal/upstream` (`Do`, `ErrorCode`, `DefaultClient`) — 5 adapters (achievements, cloudsave, playerdata, leaderboard, gameregistry) sem mais `do()`/`classify()`/`errorCode()` locais.
8. **SQS queue depth no Grafana** (observabilidade futura): visibilidade de profundidade de fila e DLQ requer `yet-another-cloudwatch-exporter` (YACE). Métricas do lado consumidor já estão no dashboard `game-lifecycle`. Implementar quando chegar em produção AWS.
9. **Service JWT** (evolução do ADR-0018): trocar a internal key compartilhada por tokens por serviço. Abrir ADR.
10. **Infra / deploy — payoff da tese (ADR-0008):** falta o grosso: demais serviços em Terraform, `environments/prod` (AWS real), compute por serviço (ADR-0005); medir custo/performance vs AWS GameKit.
11. **Data plane / o jogo (repos Unity por arquétipo, não-Go — `REPO-TOPOLOGY.md`):** o `go-game-dataplane` (queue_arena, Fases 0–7) valida o `queue_arena`; o **`go-game-host-world-dropin-dataplane`** (⬜ planejado) é o próximo — encarna o mundo do host consumindo `host_world_dropin-dataplane-contract.md` (claim `role`, join-in-progress, `ReportParticipationEvent`) + Agones. É onde a funcionalidade do drop-in vira e2e real e destrava os follow-ups do arquétipo.
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
