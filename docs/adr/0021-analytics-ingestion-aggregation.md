# ADR-0021: analytics-service ingere o fan-out via worker pool e agrega em Postgres

**Status:** Accepted
**Data:** 2026-07-04
**Serviço(s) afetado(s):** analytics-service (novo), admin-bff, admin-ui, infra
**Relacionado:** ADR-0020 (fan-out SNS→SQS), ADR-0002 (hexagonal), ADR-0005 (compute), ADR-0015 (db-per-service)

---

## Contexto

O `analytics-service` é o último serviço da trilha (≈ AWS Game Analytics Pipeline, ROADMAP Fase 9) e o único ainda não construído. O fan-out do resultado já é **SNS** (ADR-0020) justamente para que novos consumidores entrem só assinando o tópico `match-results` — o encaixe do analytics está pronto.

Decisões em aberto: **como ingerir** (o ROADMAP pede worker pool como *core*), **o que armazenar** (evento cru vs. agregado), **qual banco**, e **até onde vai a v1** (só resultado de partida vs. telemetria genérica).

Restrições: entrega **at-least-once** do SQS (precisa idempotência); o data plane ainda **não emite telemetria arbitrária** (só o `ReportResult` vira fan-out); volume de dev é baixo.

## Alternativas Consideradas

### Fonte de eventos

- **A1 — Só o fan-out `match-results` (nova fila SQS `match-results-analytics`).** Encaixe pronto, zero mudança no produtor, entrega valor já.
- **A2 — Fan-out + endpoint de ingestão genérico (`POST /events`).** Mais fiel ao Game Analytics Pipeline, mas **não há produtor de telemetria** hoje (o dataplane não emite); escopo grande sem consumidor real.

### Concorrência (ingestão)

- **B1 — Worker pool (goroutines + channels + `context`) no core.** É o objetivo didático da Fase 9; o consumidor SQS vira *driving adapter* que faz fan-in num channel, N workers agregam.
- **B2 — Loop single-goroutine** (como os consumers do fan-out, spec 006). Simples, mas não exercita o worker pool nem escala o processamento.

### Modelo de armazenamento

- **C1 — Rollups pré-agregados em Postgres (idempotente por `matchId`).** Leitura barata pro painel, storage pequeno, dedup natural. Contra: métrica nova pode exigir reprocessar/alterar schema.
- **C2 — Event store cru (append de cada evento) + agregação em query.** Flexível (métricas novas sem reprocessar), mas mais storage e queries mais caras; sem necessidade real no volume atual.

### Banco

- **D1 — Postgres** (default do ADR-0015). Volume baixo; rollups relacionais simples.
- **D2 — TSDB (Timescale/Influx).** Overkill agora; reavaliar se virar série temporal de alto volume.

## Decisão

O `analytics-service` **consome o fan-out `match-results`** por uma nova fila SQS (`match-results-analytics`, assinando o tópico existente), processa por um **worker pool** (goroutines + channels + `context`, o core hexagonal), e persiste **rollups pré-agregados por jogo em Postgres**, **idempotentes por `matchId`**. Expõe uma **read API** consumida pelo admin-bff → aba **Analytics** (overview cross-game).

Descartadas/adiadas: **A2** (telemetria genérica — sem produtor hoje), **B2** (loop simples — aqui o core é a concorrência), **C2** (event store cru — sem necessidade no volume atual), **D2** (TSDB — overkill). Gatilho para reavaliar A2/C2: o dataplane passar a emitir eventos arbitrários.

## Consequências

### Positivas

- Completa a trilha de serviços; acende a aba Analytics (hoje ComingSoon).
- Exercita **worker pool** (o padrão de concorrência de Go que faltava — contraste com `BackgroundService`/`Channel<T>` do .NET).
- Reaproveita o fan-out durável (uma assinatura a mais) e o padrão de idempotência por `matchId` (spec 006).
- Leitura barata pro painel (agregados prontos).

### Negativas / Trade-offs

- **Pré-agregação** engessa: métrica nova pode exigir migration/reprocesso (aceito; C2 é o escape futuro).
- Sem telemetria genérica, o "analytics" v1 é limitado ao que a partida reporta (`gameId`/`matchId`/`won`/`score`).
- Mais um banco + fila + consumer na stack.

### Neutras / Implicações

- Terraform (ADR-0020) ganha a fila `match-results-analytics` + DLQ + assinatura raw.
- Observabilidade: eventos recebidos/processados por s, **lag da fila** (`ApproximateNumberOfMessages`), profundidade da DLQ.
- Specs: `analytics-service/001-*` (novo) + `admin-bff/00N` + `admin-ui/011`.

## Referências

- ADR-0020 — fan-out durável SNS→SQS (o tópico que o analytics assina).
- AWS Game Analytics Pipeline — aws.amazon.com/solutions/implementations/game-analytics-pipeline
- ROADMAP Fase 9 — worker pool + goroutines/channels + ingestão via SQS.
