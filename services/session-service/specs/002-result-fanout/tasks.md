# Tasks: 002-result-fanout
**Plan:** specs/002-result-fanout/plan.md
**Data:** 2026-06-30

- [x] `internal/domain/session.go` — `ResultPublisher.Publish(ctx, gameID, results)`
- [x] `internal/publisher/log_publisher.go` — assinatura com gameID (fallback)
- [x] `internal/service/session_service.go` — `ReportResult`: release-first + `Publish(gameID)`; fan-out best-effort
- [x] `internal/service/session_service_test.go` — mock com gameID; release sempre; fan-out com gameID
- [x] `internal/publisher/achievements_publisher.go` (+ httptest) — match_played (todos) + match_won (vencedores); X-Internal-Key; best-effort
- [x] `cmd/server/main.go` — escolhe publisher (ACHIEVEMENTS_URL) + InternalKeyTransport; compose (ACHIEVEMENTS_URL, INTERNAL_API_KEY, depends_on)
- [x] `go test ./... -race` + `go build ./...`; validar no Docker (loop: ReportResult -> achievements desbloqueia)
