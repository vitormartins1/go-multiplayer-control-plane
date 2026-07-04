# Spec: 006-durable-fanout

**Serviço:** session-service (produtor) · achievements-service, leaderboard-service (consumidores) · infra
**Status:** Implementado
**Data:** 2026-07-04
**ADR:** [0020](../../../../docs/adr/0020-durable-result-fanout.md) — fan-out durável via SNS→SQS

## Contexto

Hoje o `ReportResult` faz fan-out **síncrono best-effort** (002-result-fanout, 003-leaderboard-fanout): o `Composite` chama achievements e leaderboard por HTTP e, se um falha, **loga e o resultado se perde** — sem retry nem persistência. Toleravel com chamadas de teste; com o data plane real fechando o loop (Dedicated Server ARM64 no Pi 5 reportando resultado de verdade), vira **perda de dados de produção**.

Esta feature substitui o fan-out síncrono por **SNS→SQS** (ADR-0020): o session publica o resultado **uma vez** num tópico `match-results`; cada sink consome sua própria fila SQS com **retry, DLQ e idempotência por `matchID`**. Local via **LocalStack** (mesmo padrão SDK+endpoint do MinIO/S3, ADR-0004); AWS real em prod. O Transactional Outbox fica **adiado** (ADR-0020, seção dedicada).

## Comportamento

### Produtor (session-service)

- WHEN `ReportResult{matchId, results[]}` -> THE SYSTEM SHALL publicar **uma** mensagem no tópico `match-results` com o **resultado bruto** (`gameID`, `matchID`, `results[]{playerID, won, score}`), e então encerrar a sessão e liberar o game server.
- IF a publicação no tópico falhar -> THE SYSTEM SHALL retornar erro no `ReportResult` (para o **retry do Dedicated Server**, Fase 6 do data plane) e **não** encerrar a sessão — o TTL sweeper existente cobre a sessão órfã. *(Muda o best-effort de 002/003: publicar deixa de ser silencioso — falha propaga.)*
- IF não existir sessão para o `matchId` -> `NotFound` (inalterado).
- THE SYSTEM SHALL parar de traduzir o resultado em eventos: a **tradução migra para os consumidores** (o tópico carrega o resultado bruto, não eventos por sink).

### Consumidor de achievements

- WHEN chega uma mensagem na fila `match-results-achievements` -> THE SYSTEM SHALL, para cada `result`, aplicar `match_played` (todos) e `match_won` (quando `won=true`) no jogo da mensagem — o mesmo efeito da 002, agora do lado consumidor.
- THE SYSTEM SHALL ser **idempotente por `matchID`**: reprocessar a mesma partida (entrega at-least-once do SQS) **não** altera contagens além da primeira vez.
- THE SYSTEM SHALL confirmar (ACK/delete) a mensagem **somente após sucesso**; em falha, a mensagem volta à fila e, após N tentativas, vai para a **DLQ** (sem travar as demais).

### Consumidor de leaderboard

- WHEN chega uma mensagem na fila `match-results-leaderboard` -> THE SYSTEM SHALL submeter o `score` de cada `result` no jogo da mensagem — o mesmo efeito da 003.
- THE SYSTEM SHALL ser **idempotente por `matchID`** e seguir a mesma disciplina de ACK-após-sucesso + DLQ.

### Infra / configuração

- THE SYSTEM SHALL provisionar via Terraform: 1 tópico SNS `match-results`, 2 filas SQS (`-achievements`, `-leaderboard`) subscritas ao tópico, e 1 DLQ por fila com redrive policy.
- THE SYSTEM SHALL usar endpoint configurável (`AWS_ENDPOINT_URL` → LocalStack no local, vazio → AWS real), como o MinIO/S3 já faz.
- IF nenhum tópico/fila estiver configurado (envs ausentes) -> o session cai no `LogPublisher` de fallback (dev sem LocalStack), e os consumidores não sobem.

## Contratos de Interface

- **Envelope da mensagem** (JSON, versionado): `{ "schemaVersion": 1, "matchId", "gameId", "results": [{ "playerId", "won", "score" }] }`.
- **`SNSPublisher`** (`session-service/internal/publisher/sns_publisher.go`) satisfaz `domain.ResultPublisher` (`Publish` passou a receber `matchID`, chave de idempotência no envelope) — serializa o envelope e faz `sns.Publish`. Substitui o `Composite` no wire-up do `main.go`. `LogPublisher` segue como fallback.
- **Consumidores** — um binário novo por serviço: `achievements-service/cmd/consumer` e `leaderboard-service/cmd/consumer`. Fazem long-poll na SQS, desserializam o envelope, aplicam o efeito reusando o **service** existente (in-process, não via HTTP admin) e deletam a mensagem no sucesso.
- **Idempotência** — achievements via tabela `processed_matches` numa **única tx** com o progresso (dedup por `matchID`). Leaderboard **não precisa de dedup**: o `Submit` usa `ZADD GT`, então reprocessar é no-op intrínseco.
- Os adapters HTTP `achievements_publisher.go` / `leaderboard_publisher.go` e o `composite.go` do session **são removidos** (a tradução vive agora nos consumidores).

## Observabilidade (obrigatória — ver docs/OBSERVABILITY.md)

- Produtor: contador `fanout_publish_total{result="ok|error"}`.
- Cada consumidor: `fanout_consume_total{result="ok|dup|error"}`, latência de processamento, e profundidade da **DLQ** exposta como métrica/alerta.
- Consumidores entram na stack (scrape no Prometheus; tracing OTel propagado pelo envelope, se viável).

## Casos de Teste Obrigatórios

- [x] `SNSPublisher` serializa o envelope correto e publica no tópico; erro de publish -> `ReportResult` retorna erro e **não** encerra a sessão
- [x] `matchId` inexistente -> `NotFound`
- [x] consumidor de achievements aplica `match_played`/`match_won` por jogador a partir do envelope
- [x] consumidor de leaderboard submete `score` por jogador
- [x] **idempotência**: entregar a mesma mensagem 2× não dobra contagem/score (dedup por `matchID`)
- [x] falha de processamento -> mensagem não é deletada; após N tentativas vai para a DLQ
- [x] fallback: sem envs de SNS/SQS o session usa `LogPublisher` (dev)

## Fora do escopo

- **Transactional Outbox** — adiado (ADR-0020, com gatilhos de reintrodução).
- **analytics-service** como consumidor — feature futura; só assinará o tópico `match-results` (motiva o SNS).
- **Exactly-once** — entrega é at-least-once + idempotência; sem coordenação distribuída.
- **Terraform de prod (AWS real)** — aqui só o módulo + LocalStack local; o apply em AWS entra na fase de infra/deploy (ADR-0008).
- Mapeamento rico de eventos (kills, tempo) — segue `match_played`/`match_won` + `score`.
