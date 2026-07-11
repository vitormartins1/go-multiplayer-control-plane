# Spec: 001-match-analytics

**Serviço:** analytics-service (novo)
**Status:** Done
**Data:** 2026-07-04
**ADR:** [0021](../../../../docs/adr/0021-analytics-ingestion-aggregation.md)
**Parte de:** Analytics (pares: `admin-bff/009-analytics`, `admin-ui/011-analytics`)

## Contexto

Último serviço da trilha (≈ AWS Game Analytics Pipeline, ROADMAP Fase 9). Ingere o **resultado das partidas** pelo fan-out durável (ADR-0020): assina o tópico SNS `match-results` por uma nova fila SQS `match-results-analytics`. O ingest é processado por um **worker pool** (o *core* hexagonal — goroutines/channels/context, ADR-0002/0021) e vira **rollups pré-agregados por jogo** em Postgres, **idempotentes por `matchId`** (at-least-once do SQS). Expõe uma **read API** para o painel.

Fonte v1 = só o `match-results` (envelope `{gameId, matchId, results[{playerId, won, score}]}`); telemetria genérica adiada (ADR-0021).

## Comportamento

### Ingestão (worker pool)

- WHEN chega uma mensagem na fila `match-results-analytics` -> o **consumidor SQS** (driving adapter) desserializa o envelope e entrega ao **worker pool** (fan-in por channel; N workers).
- THE SYSTEM SHALL agregar cada partida de forma **idempotente por `matchId`** (reprocessar não recontabiliza), numa transação:
  - `matches_total += 1` no jogo; `matches` do **dia** (UTC) += 1;
  - por player do resultado: `participations += 1`, `wins += (won?1:0)`, `score_sum += score`; registra o player como **visto** no jogo (para jogadores únicos).
- THE SYSTEM SHALL confirmar (delete) a mensagem só após sucesso; falha -> volta à fila -> DLQ (mesma disciplina da spec 006).
- THE SYSTEM SHALL encerrar o worker pool de forma limpa no shutdown (`context` cancelado + `WaitGroup`), drenando o que estiver em processamento.

### Read API (admin, sob `/admin` — internal key na borda, ADR-0018)

- WHEN `GET /admin/analytics/overview` -> THE SYSTEM SHALL devolver o **panorama cross-game**: totais (partidas, jogadores únicos, jogos com atividade) + lista de jogos com `{gameId, matches, uniquePlayers, wins, avgScore}`, ordenada por `matches` desc.
- WHEN `GET /admin/games/{gameId}/analytics` -> THE SYSTEM SHALL devolver o resumo do jogo (`matches, uniquePlayers, participations, wins, avgScore`) + série **`daily`** (`[{day, matches}]`, últimos N dias).
- `avgScore = score_sum / participations` (0 quando sem participações). Jogo sem dados -> zeros/listas vazias (200, não 404).

### Observabilidade (obrigatória)

- `analytics_events_received_total` / `analytics_events_processed_total{result="ok|dup|error"}`; duração de processamento; **workers ativos** (gauge). Lag/DLQ da fila como no fan-out (spec 006). `/metrics` + `/healthz`; entra no Prometheus. Tracing OTel + otelpgx.

## Contratos de Interface

- **Estrutura** (hexagonal, ADR-0002): `internal/domain` (tipos + Ports), `internal/service` (**worker pool** + agregação), `internal/repository` (Postgres), `internal/transport` (HTTP read API), `internal/consumer` (SQS driving adapter → channel), `internal/awsx` (réplica, decisão 1a da 006).
- **Envelope** consumido: o mesmo publicado pelo session (`schemaVersion/matchId/gameId/results[]`) — struct próprio do analytics (desacoplado).
- **Migrations:** `processed_matches(match_id pk, game_id)`; `game_rollup(game_id pk, matches_total, participations, wins, score_sum)`; `game_player_seen(game_id, player_id, pk composto)` (jogadores únicos); `game_daily(game_id, day, matches, pk (game_id,day))`.
- **Infra:** Terraform ganha a fila `match-results-analytics` + DLQ + assinatura raw ao tópico; compose ganha `analytics-postgres` (+ exporter), `analytics-service` e o worker consumidor no mesmo binário.

## Casos de Teste Obrigatórios

- [ ] agregação de uma partida: `matches_total`, participações, wins, score_sum, daily e jogadores únicos corretos
- [ ] **idempotência**: reprocessar o mesmo `matchId` não recontabiliza (dedup transacional)
- [ ] jogadores únicos: o mesmo player em 2 partidas conta 1 em `uniquePlayers`, 2 em participações
- [ ] `overview` ordena por matches desc e soma os totais; jogo sem dados -> zeros
- [ ] worker pool: N mensagens concorrentes agregam sem corrida (teste com `-race`); shutdown drena e encerra
- [ ] consumidor: sucesso deleta; erro não deleta (DLQ)

## Fora do escopo

- **Telemetria genérica** (`POST /events`) e **event store cru** — adiados (ADR-0021, com gatilho).
- Métricas derivadas ricas (retenção, funis, DAU/MAU reais) — v1 fica no que a partida reporta.
- Séries de alta cardinalidade / TSDB — Postgres basta no volume atual.
