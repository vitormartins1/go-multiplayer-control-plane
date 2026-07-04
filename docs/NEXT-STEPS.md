# NEXT-STEPS — handoff (atualizado 2026-07-05)

Ponto de retomada após limpar o contexto. Resume **o estado atual**, **o que fica pendente de você**, **o que foi feito nesta sessão** e o **backlog priorizado**.

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

Commit `1947ce4` na `main`:

- **Review de observabilidade Grafana**: levantamento completo (serviços × dashboards × exporters) — nenhum exporter faltando; lacunas identificadas foram métricas customizadas do matchmaking/session/analytics sem dashboard.
- **Dashboard `game-lifecycle.json`**: funil completo — fila de matchmaking, matches formados/falhas, sessões ativas, sessões encerradas por motivo, consumo fan-out (achievements + leaderboard consumers) e latência p95 do processamento.
- **Dashboard `analytics-workers.json`**: workers ativos, ingest vs processamento (eventos/s), resultado por linha (ok/dup/error), backpressure.
- **Importação CSV de conquistas** (specs `admin-bff/011` + `admin-ui/014`): feature completa — BFF orquestra criações sequenciais com resultado por linha; parser CSV nativo no frontend; validação pré-envio (regex, campos obrigatórios, IDs duplicados no arquivo, ≤ 500 linhas, ≤ 200 KB); modais de erro e resultado. 9 testes unitários novos (4 no client, 5 no handler).

*Sessões anteriores (histórico):*

- `player-data/002-game-scope`, `cloud-save/002-game-scope`, `admin-bff/006-player-tabs`, `admin-ui/005-game-library`, `game-registry/002-004`, `achievements/006-catalog-crud`, chore module path + CI + code-splitting, code-review `high`.

## Backlog priorizado (sem spec ainda)

Uma feature por branch `feat/<serviço>/<NNN-slug>`, merge `--no-ff` (`docs/GIT-WORKFLOW.md`). Ordem sugerida:

1. **Push pro GitHub** (acima) → CI passa a rodar. **Atenção:** o CI agora precisa de Docker p/ os testes de integração/e2e do fan-out (LocalStack + Postgres via testcontainers).
2. ✅ **Fan-out resiliente — FEITO** (ADR-0020, spec `session/006-durable-fanout`).
3. ✅ **analytics-service — FEITO** (ADR-0021). *Evoluções v1→v2 quando fizer sentido:* telemetria genérica (`POST /events`); deep-dive por jogo (aba no GameDetail). O **event store cru** (C2 do ADR-0021) é o escape se a pré-agregação engessar.
4. ✅ **Importação CSV de conquistas — FEITA** (specs `admin-bff/011` + `admin-ui/014`, commit `1947ce4`).
5. **Remover jogo** no painel — *rename* ✅ feito. *Delete* segue **adiado** por precisar de decisão sobre dado órfão (cascata cross-service). **Precisa de ADR** — opções: *soft delete* (mais barato) vs *hard cascade*. A aba Configurações já avisa "remover ainda não disponível".
6. **Leaderboard**: seasons/janelas por período + paginação.
7. **Cleanup admin-bff** (do code-review, não é bug): `do()`/`classify()`/`errorCode()`/default-client duplicados nos 4 adapters → extrair helper `internal/upstream`.
8. **SQS queue depth no Grafana** (observabilidade futura): visibilidade de profundidade de fila e DLQ requer `yet-another-cloudwatch-exporter` (YACE). Métricas do lado consumidor já estão no dashboard `game-lifecycle`. Implementar quando chegar em produção AWS.
9. **Service JWT** (evolução do ADR-0018): trocar a internal key compartilhada por tokens por serviço. Abrir ADR.
10. **Infra / deploy — payoff da tese (ADR-0008):** falta o grosso: demais serviços em Terraform, `environments/prod` (AWS real), compute por serviço (ADR-0005); medir custo/performance vs AWS GameKit.
11. **Data plane / o jogo (repo `go-game-unity`, não-Go):** Unity Dedicated Server + Netcode + Agones/EKS (ADR-0010/0011/0012).

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
