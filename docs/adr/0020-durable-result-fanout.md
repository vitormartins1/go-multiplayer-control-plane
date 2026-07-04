# ADR-0020: Fan-out durável do resultado via SNS→SQS (LocalStack local, AWS prod)

**Status:** Accepted
**Data:** 2026-07-04
**Serviço(s) afetado(s):** session-service, achievements-service, leaderboard-service, infra
**Relacionado:** ADR-0004 (Terraform/LocalStack), ADR-0008 (compute commodity), ADR-0005 (compute por serviço)

---

## Contexto

O `session-service` recebe o resultado da partida do Dedicated Server (Unity) via `ReportResult` (gRPC) e faz **fan-out** para os sinks — hoje achievements e leaderboard — através de um `Composite` que chama cada `ResultPublisher` **síncrono e best-effort**: se um sink falha, loga um `Warn` e segue; o resultado para aquele sink **é perdido**, sem retry nem persistência.

Isso era tolerável enquanto o loop era exercitado por chamadas de teste. Com o data plane real fechando o loop (`go-game-dataplane` Fases 5–7: Dedicated Server ARM64 orquestrado por Agones no homelab Pi 5, reportando resultado de verdade), a fragilidade vira **perda de dados de produção**: uma partida termina, um sink está fora por segundos, e a conquista/ranking daquele jogador some.

Restrições relevantes:

- O `session-service` é **in-memory por design** (sessões ativas em mapa + Agones como fonte de verdade da alocação). **Não tem Postgres** (`internal/`: `allocator/domain/publisher/service/token/transport`, sem `repository` nem `migrations`).
- O projeto já tem um **padrão local-first consolidado** para infra AWS: `cloud-save-service` usa `aws-sdk-go-v2/service/s3` contra **MinIO** local, trocando só o `BaseEndpoint` para a AWS real (ADR-0004). SQS/SNS são infra **commodity** (não são serviços purpose-built de jogo), logo permitidos pela tese do ADR-0008 — o próprio CLAUDE.md já lista SQS na stack.
- Uma feature futura próxima (`analytics-service`) será um **novo consumidor** do mesmo resultado.

## Alternativas Consideradas

### Opção A: SNS → SQS via LocalStack (local) / AWS (prod) ✅

Um tópico SNS `match-results`; cada sink tem sua própria fila SQS assinando o tópico, com **DLQ** e redrive.

- **Prós:** paridade 1:1 com AWS (mesmo SDK, só muda endpoint — idêntico ao MinIO/S3); fan-out durável nativo; cada sink isolado com retry/visibility-timeout/DLQ próprios (um sink lento não trava os outros); **extensível de graça** — analytics só assina o tópico, zero mudança no producer; exercita AWS SDK Go v2 + padrão consumer.
- **Contras:** at-least-once → exige idempotência por `match_id` nos consumers; LocalStack tem divergências de borda vs AWS; introduz LocalStack + 2 pequenos binários consumers.

### Opção B: SQS única (sem SNS), consumer faz o fan-out

- **Prós:** uma peça a menos.
- **Contras:** o desacoplamento por sink se perde (ou o consumer vira um novo ponto único de fan-out); adicionar analytics exige mexer no consumer. Descartada por matar a extensibilidade que é metade do valor.

### Opção C: Redis Streams (Redis já está no compose)

- **Prós:** infra zero (já usado pelo leaderboard); consumer groups + ACK + retry.
- **Contras:** **nenhuma paridade AWS** — Redis não é um serviço de fila da AWS, quebra a comparação 1:1 que é o entregável de tese (ADR-0008). Fan-out manual.

### Opção D: NATS JetStream / RabbitMQ

- **Prós:** ótimos brokers, NATS é idiomático em Go.
- **Contras:** broker novo, **sem par AWS gerenciado direto** (fura a tese). Over-engineering para o volume atual.

### Opção E: Transactional Outbox (tabela no Postgres do session) + poller

Grava resultado + marcação de entrega numa única transação; um poller entrega aos sinks (ou publica no SNS).

- **Prós:** atomicidade real, elimina o dual-write, durabilidade independente do broker.
- **Contras:** **exige introduzir Postgres no session-service** (hoje in-memory) — migrations, exporter obrigatório (regra de observabilidade), pgx/otelpgx, poller com locking/backoff/cleanup. Escopo grande. **Ver a seção dedicada abaixo.**

## Decisão

Adotamos a **Opção A: fan-out do resultado via SNS→SQS**, emulado por **LocalStack** no ambiente local e **AWS real** em produção, seguindo o mesmo padrão "SDK AWS + endpoint configurável" já usado pelo MinIO/S3 (ADR-0004).

- O `Composite` (best-effort síncrono) é substituído por um `SNSPublisher` que implementa a mesma Port `domain.ResultPublisher` — encaixe cirúrgico no wire-up do `session-service`.
- Cada sink (achievements, leaderboard) vira um **consumer** de sua própria fila SQS, com **retry/backoff, DLQ e idempotência por `match_id`**.
- O `session-service` publica **antes** de fechar a sessão/liberar o server; o cleanup do server é idempotente e coberto pelo TTL sweeper existente.

**Adiamos o Transactional Outbox (Opção E)** — justificativa na seção abaixo.

## Outbox table: agora ou depois?

Análise dedicada porque é o principal trade-off desta decisão.

**O que o outbox resolveria (a favor de agora):**

- **Atomicidade real:** o commit do banco vira o único ponto de verdade — grava resultado e "marca para entregar" juntos. Elimina o *dual-write* (publicar num broker é um segundo sistema; se o processo morre entre persistir e publicar, perde).
- **Durabilidade independente do broker:** se o SNS/LocalStack estiver fora no instante do report, o resultado já está no banco e o poller entrega quando voltar.
- **Auditoria/replay:** histórico de resultados no banco — útil para o futuro analytics e para depurar.

**O que custa ter outbox agora (contra):**

- **Dá Postgres a um serviço deliberadamente in-memory.** Puxa migrations, `session-postgres` + exporter (obrigatório pela regra de observabilidade), pool pgx, otelpgx, testcontainers, wire-up — muda o modelo de deploy/escala do session, que hoje escala trivialmente com Agones como fonte de verdade.
- **Complexidade do poller:** loop de polling, backoff, locking (não entregar 2×), marcação de status e cleanup das linhas entregues.
- **Ganho marginal estreito aqui:** o "segundo write" após publicar (fechar sessão/liberar server) **não precisa ser atômico** com o resultado — é cleanup idempotente já coberto pelo TTL sweeper. O único cenário de perda real é *SNS fora **e** os 3 retries do Dedicated Server (Fase 6) esgotados* — raro, e o LocalStack local praticamente não cai. É o **mesmo risco residual de hoje**, não um novo.

**Decisão:** adiar. O SNS→SQS + DLQ + idempotência **já elimina a fragilidade atual** (fan-out best-effort sem retry, onde um sink fora perde o dado). O resíduo que só o outbox cobriria é raro e não justifica introduzir um banco agora.

**Gatilhos para reintroduzir o outbox (reavaliar quando qualquer um ocorrer):**

1. O session passar a **persistir sessões/partidas em Postgres** por outro motivo (ex.: histórico para o analytics) — aí o outbox entra "de graça" na transação que já existirá.
2. Requisito de **zero-perda auditável / exactly-once** (ex.: se o resultado passar a mover moeda/economia).
3. **Perda real observada** em métrica (`fanout_publish_failures_total` acima de zero de forma não-transitória).

## Consequências

### Positivas

- Fan-out durável com isolamento por sink (DLQ), resolvendo o ponto mais frágil do sistema.
- Extensível: `analytics-service` só assina o tópico `match-results`.
- Mantém a comparação de tese (SNS/SQS local vs AWS — só muda endpoint) e reaproveita o padrão MinIO já dominado.
- Exercita AWS SDK Go v2 + padrão consumer/idempotência (aquece o terreno para o analytics).

### Negativas / Trade-offs

- Entrega at-least-once → **idempotência por `match_id` obrigatória** nos consumers.
- LocalStack adiciona uma peça ao compose e pode divergir da AWS em bordas.
- Sem atomicidade produtor↔broker (aceito conscientemente; ver seção do outbox).

### Neutras / Implicações

- Terraform (ADR-0004) passa a provisionar SNS+SQS+DLQ por ambiente.
- Achievements e leaderboard ganham um binário consumer cada (entram na observabilidade — métricas RED do consumer, lag da fila).
- Provável spec SDD: `session-service/00N-durable-fanout` (+ consumers), em `feat/session-service/...`.

## Referências

- ADR-0004 — Terraform/LocalStack (padrão SDK AWS + endpoint configurável, ex.: MinIO/S3).
- ADR-0008 — compute commodity (SQS/SNS são infra genérica, não serviço-de-jogo).
- Transactional Outbox — microservices.io/patterns/data/transactional-outbox.html
- LocalStack SNS/SQS — docs.localstack.cloud
