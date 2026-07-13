# analytics-service

≈ **AWS Game Analytics Pipeline**. Consome o fan-out do resultado de partida (SNS → SQS, ADR-0020), agrega **rollups pré-agregados por jogo em Postgres** através de um **worker pool** (goroutines + channels + `context`, ADR-0021) e expõe uma **read API admin** consumida pelo admin-ui (aba **Analytics**).

- **Protocolo consumer:** SQS (long-poll, `match-results-analytics`) · **API admin:** REST · **Porta host:** 8091 (container `:8080`)
- **Storage:** Postgres (`analytics`), rollups idempotentes — não é event store
- **Dependências:** SQS/SNS (LocalStack em dev), Postgres, admin-bff (agrega a leitura pro painel)

O ADR original (0021) desenhou a v1 só para o desfecho terminal `win_loss_draw` do `queue_arena`. O código foi além: hoje o consumer despacha **4 tipos de envelope** do mesmo tópico `match-results`, um por família de arquétipo/resultado — a granularidade real do domínio é **jogo → modo → papel → evento → outcome**, não só "partida ganha/perdida".

## Envelopes suportados

O consumer (`internal/consumer/consumer.go`) espia o campo `kind` do envelope (ou cai no fallback por `matchId`, envelope v1 legado) e despacha para o caminho do pool correspondente:

| `kind` | Arquétipo/origem | Payload agregado | Unidade do pool |
|---|---|---|---|
| `win_loss_draw` (ou ausente, v1 legado) | `queue_arena` (ADR-0023) — `ReportResult` | resultado por jogador: `outcome` (win/loss/draw), `score` | `MatchResult` → `Submit` |
| `participation_event` | `host_world_dropin` (session/013, ADR-0024) — `ReportParticipationEvent` | evento por participante: `role`, `event`, `outcome`, `modeId`, `eventId` | `ParticipationEvent` → `SubmitParticipation` |
| `placement` | `battle_royale` (session/015) | colocação ordinal + tempo de sobrevivência por participante (sem `playerId`) | `PlacementResult` → `SubmitPlacement` |
| `collective` | `coop_mission` (session/016) | desfecho **coletivo** da sessão: `MISSION_SUCCESS`/`MISSION_FAILURE` (ignora players/stats) | `CollectiveResult` → `SubmitCollective` |

Envelopes com `kind` desconhecido e sem `matchId` (ex.: `summon_posted`, eventos de outros consumidores do mesmo tópico) são descartados (mensagem deletada, não vai à DLQ). Cada caminho tem sua própria idempotência (ver Modelo de dados) e sua própria goroutine de submissão no pool — o fan-in dos resultados só deleta da fila as mensagens que agregaram com sucesso; falha real devolve a mensagem à fila (at-least-once → DLQ após `maxReceiveCount`).

## API admin

Sem auth própria no serviço (`/admin` fica atrás da internal key na borda, ADR-0018). Consumida pelo admin-bff → aba Analytics do admin-ui.

| Método | Rota | Descrição |
|---|---|---|
| GET | `/admin/analytics/overview` | Panorama **cross-game**: totais (partidas, jogadores únicos, jogos ativos), rollup por jogo, série "partidas por dia" somada entre todos os jogos (últimos 30 dias) |
| GET | `/admin/games/{gameId}/analytics?days=N` | Detalhe de **um jogo**: rollup + série diária dos últimos `N` dias (default 30, máx. 365) + `roleParticipations` (host_world_dropin) + `placement` (battle_royale, omitido fora dele) + `mission` (coop_mission, omitido fora dele) + `placementDaily`/`missionDaily`/`participationDaily` (série diária dos 3 arquétipos que só tinham rollup cumulativo, spec 010) + `stats` (resumo agregado — soma/contagem/média — de stats brutos por `stat_key`, spec 011) |

`roleParticipations` é a quebra por `(modeId, role, event, outcome, count)` — vazia em jogos `queue_arena`. `placement`/`mission` são `null` (omitidos no JSON) para jogos fora do arquétipo correspondente — campos aditivos, não union type. `placementDaily`/`missionDaily`/`participationDaily` e `stats` seguem o mesmo padrão aditivo (listas vazias, nunca `null`).

## Modelo de dados

Postgres, rollups incrementais (não há tabela de eventos crus). Cada família de envelope tem sua própria tabela de idempotência + rollup:

| Tabela | Papel |
|---|---|
| `processed_matches` (migration 001) | idempotência por `match_id` — cobre `win_loss_draw`, `placement` e `collective` (todos têm `matchId` único por partida/sessão) |
| `game_rollup` (001, +`losses`/`draws` na 002) | rollup por jogo do `queue_arena`: `matches_total`, `participations`, `wins`, `losses`, `draws`, `score_sum` (`avgScore` derivado no read) |
| `game_player_seen` (001) | jogadores distintos vistos por jogo (dedup natural via PK composta) — base do "jogadores únicos" |
| `game_daily` (001) | série "partidas por dia" (bucket UTC) por jogo |
| `processed_participations` (003, chave trocada na 006) | idempotência por `event_id` (host_world_dropin) — antes de 006 era `(session_id, player_id, event)`, insuficiente pra evento repetível na mesma sessão (ex. boss que respawna) |
| `game_participation_rollup` (003; +`outcome` na 006; +`mode_id` na 007) | contagem por `(game_id, mode_id, role, event, outcome)` — o funil de invasão/co-op do host_world_dropin, com outcome e modo nomeado como dimensões |
| `game_placement_rollup` (004) | histograma de colocação do battle_royale por jogo: `winners` (1º), `top_n` (2..10, `BattleRoyaleTopNThreshold`), `rest`, `entries` (denominador), `sum_survived_ms` (`avgSurvivedMs` derivado no read) |
| `game_mission_rollup` (005) | contagem `successes`/`failures` do coop_mission por jogo (`successRate` derivado no read) |
| `game_placement_daily` / `game_mission_daily` / `game_participation_daily` (008, spec 010) | série "por dia" por `(game_id, mode_id, day)` — os 3 arquétipos que só tinham rollup cumulativo (sem dimensão de tempo) até esta spec. `game_participation_daily` conta EVENTOS (`events`), não sessões — host_world_dropin não tem partida terminal |
| `game_player_stats` (009, spec 011) | stats brutos write-many, uma linha por `(game_id, mode_id, match_id, player_id, stat_key)` — captura os 4 kinds do fan-out (incl. `participation_event`, destravado por `session-service/026`). Sem UNIQUE própria: idempotência herdada da mesma tx do rollup/`processed_*` |

Dados agregados antes de uma dimensão existir caem em bucket "vazio" explícito (`outcome=''`, `mode_id=''`) em vez de serem perdidos ou reconstituídos — mesmo padrão nas migrations 006 e 007. As tabelas diárias novas (008) já nascem com `mode_id` na PK — os rollups cumulativos (`game_rollup`/`game_placement_rollup`/`game_mission_rollup`) continuam sem essa dimensão (fora de escopo da spec 010, deliberado).

## Observabilidade

Prometheus: `analytics_events_received_total` (partidas/eventos submetidos ao pool), `analytics_events_processed_total{result}` (ok/dup/error), `analytics_workers_active` (gauge de workers ocupados). `/metrics` na porta do serviço (**Service Overview**, `$job=analytics-service`). Tracing no Tempo (pgx instrumentado via `otelpgx`). Lag da fila (`ApproximateNumberOfMessages`) e profundidade da DLQ (`match-results-analytics-dlq`) via CloudWatch/LocalStack — ver ADR-0020.

Sem `ANALYTICS_QUEUE_URL` configurada, o serviço sobe só a read API (sem ingestão) — loga aviso no boot, não falha.

## Testar / Specs

`services/analytics-service/specs/` — 9 specs, agrupadas por tema:

- **Base (`queue_arena`):** `001-match-analytics` (ingestão v1 + rollup por jogo), `002-overview-daily` (série diária + overview cross-game), `003-outcome-aggregation` (ADR-0023: `outcome` polimórfico — win/loss/draw em vez de só `won`)
- **host_world_dropin:** `004-participation-analytics` (rollup de eventos de participação por papel), `007-participation-event-id-and-outcome` (idempotência por `eventId` + `outcome` na chave do rollup), `008-participation-outcome-dto` (expõe `outcome` no DTO admin — a query já agregava, o DTO não repassava), `009-participation-mode-breakdown` (quebra por `modeId`)
- **battle_royale:** `005-battle-royale-placement` (envelope `placement`, histograma de colocação + sobrevivência)
- **coop_mission:** `006-coop-mission-collective` (envelope `collective`, desfecho coletivo da missão)

Sem `api/*.http` neste serviço (só admin, sem endpoint de jogador) — testar via `docker compose up analytics-service` + `curl localhost:8091/admin/analytics/overview` (fluxo completo exige o fan-out real: publicar em `match-results` no LocalStack e aguardar o consumer processar).
