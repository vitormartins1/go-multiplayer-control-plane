# Plan: 002-result-fanout
**Spec:** specs/002-result-fanout/spec.md
**Data:** 2026-06-30

## Abordagem técnica
Troca a Port `ResultPublisher.Publish(ctx, matchID, results)` por `Publish(ctx, gameID, results)`. Novo adapter HTTP (`AchievementsPublisher`) chama `POST /admin/games/{gameID}/players/{playerID}/events` por evento, com a internal key via `auth.InternalKeyTransport` (ADR-0018). `ReportResult` libera o server **antes** do fan-out e trata o fan-out como **best-effort** (loga, não falha o report nem vaza server).

## Arquivos
- `internal/domain/session.go` — `ResultPublisher.Publish(ctx, gameID string, results []PlayerResult)`
- `internal/service/session_service.go` — `ReportResult`: release primeiro, depois `Publish(ctx, sess.GameID, results)`; fan-out não bloqueia o encerramento
- `internal/service/session_service_test.go` — mock Publish com gameID; assert release sempre; fan-out chamado com gameID
- `internal/publisher/achievements_publisher.go` (+ test, httptest) — para cada result: POST `match_played`; se `won`, POST `match_won`; best-effort (loga falhas, retorna nil)
- `internal/publisher/log_publisher.go` — ajustar assinatura (gameID); segue como fallback
- `cmd/server/main.go` — lê `ACHIEVEMENTS_URL` + `INTERNAL_API_KEY`; se `ACHIEVEMENTS_URL` setado, usa `AchievementsPublisher` (client com InternalKeyTransport); senão `LogPublisher`
- `infra/docker/docker-compose.yml` — session: `ACHIEVEMENTS_URL` + `INTERNAL_API_KEY` + `depends_on: achievements-service`
- `api/README.md` — nota do fan-out

## Decisões
- **Best-effort**: o report do server é ack mesmo se o achievements oscilar; loga as falhas. Retry/outbox é evolução.
- **Release antes do fan-out**: corrige o risco de vazar server se o publish falhar.
- **Eventos `match_played` + `match_won`** (decisão da spec): simples e extensível.
- **Fallback LogPublisher**: sem `ACHIEVEMENTS_URL`, degrada para log (dev).

## Ordem
1. domain (Port assinatura) + log_publisher ajustado
2. service ReportResult (release-first + gameID) + test
3. AchievementsPublisher + httptest
4. main (escolha do publisher + InternalKeyTransport) + compose
5. `go test -race` + build; validar no Docker o loop: CreateSession+ReportResult -> achievements desbloqueia

## Riscos
- **Best-effort vs visibilidade**: logar bem as falhas de fan-out (slog).
- **N eventos por partida**: poucas chamadas (players * 1-2); ok síncrono no MVP.
- **Ordem release/fan-out**: garantir release sempre.
