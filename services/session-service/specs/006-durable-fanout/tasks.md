# Tasks: 006-durable-fanout

**Plan:** specs/006-durable-fanout/plan.md
**Data:** 2026-07-04

> Duas decisões abertas no plan (§Decisões abertas) devem ser confirmadas antes das tasks 1 e 9.

## Produtor — session-service

- [x] `domain` — estender `ResultPublisher.Publish(ctx, gameID, matchID, results)`; ajustar `LogPublisher` e fakes de teste
- [x] `internal/publisher/sns_publisher.go` (+ test) — serializa envelope `{schemaVersion:1, matchId, gameId, results[]}`, `sns.Publish`; client via helper AWS
- [x] `internal/service/session_service.go` — `ReportResult` passa `matchID`; **publish falha → erro, não encerra sessão** (TTL sweeper cobre); testar o caminho de erro
- [x] `cmd/server/main.go` — `buildPublisher`: `MATCH_RESULTS_TOPIC_ARN` → `SNSPublisher`, senão `LogPublisher`; remover ramo Composite/HTTP
- [x] remover `internal/publisher/{achievements_publisher,leaderboard_publisher,composite}.go` (+ testes) e envs de fan-out HTTP

## Consumidor — achievements-service (dedup por matchID)

- [x] `migrations/004_processed_matches.sql` — `processed_matches(match_id pk, processed_at)`
- [x] `internal/repository` — métodos aceitam querier/`pgx.Tx` (executor injetável)
- [x] `internal/service/achievement_service.go` — `ApplyMatchResult(ctx, gameID, matchID, results)`: tx única (INSERT ON CONFLICT → dup=no-op; senão `match_played`/`match_won` por player) (+ test de idempotência: 2× = 1 efeito)
- [x] `cmd/consumer/main.go` — long-poll SQS, `ApplyMatchResult`, delete-após-sucesso, DLQ por não-delete; `context`+`WaitGroup`; `/metrics`+`/healthz`
- [x] métricas `fanout_consume_total{result}` + duração

## Consumidor — leaderboard-service (idempotência intrínseca via ZADD GT)

- [x] `cmd/consumer/main.go` — long-poll SQS, `service.Submit` por player, delete-após-sucesso, DLQ; `context`+`WaitGroup`; `/metrics`+`/healthz`; **sem dedup** (comentar: highscore GT é idempotente)
- [x] test: reentrega da mesma mensagem não altera o top-N (no-op)
- [x] métricas `fanout_consume_total{result}` + duração

## Infra + observabilidade

- [x] helper de config AWS (decisão §1: `internal/awsx` por serviço, ou `shared/awsx` com OK)
- [x] provisionamento SNS+SQS+DLQ+subscriptions (decisão §2: Terraform local, ou init do LocalStack)
- [x] `infra/docker/docker-compose.yml` — `localstack` (sns,sqs) + healthcheck; `session` ganha `MATCH_RESULTS_TOPIC_ARN`/`AWS_ENDPOINT_URL`; serviços `achievements-consumer`/`leaderboard-consumer`
- [x] `infra/docker/prometheus.yml` — jobs dos dois consumers (`docker compose restart prometheus`)
- [x] Dockerfile(s) — compilar o binário `cmd/consumer` além do `cmd/server`

## Fechamento

- [x] `go test ./... -race -cover` (≥80% em service/domain) + `go build ./...` + `golangci-lint run`
- [x] validar o loop no Docker: `ReportResult` → SNS → 2 filas; achievements/leaderboard refletem; matar consumer → mensagem processa ao voltar; forçar erro → DLQ
- [x] atualizar `docs/UNITY-INTEGRATION.md` / `docs/OBSERVABILITY.md` se o contrato/observabilidade mudar
- [x] `docs/GO-LEARNINGS.md` — registrar o que apareceu de Go novo (SQS long-poll + graceful shutdown, tx pgx p/ idempotência, envelope desacoplado); resumo didático ao usuário
