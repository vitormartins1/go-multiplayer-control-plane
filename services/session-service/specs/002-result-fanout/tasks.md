# Tasks: 002-result-fanout
**Plan:** specs/002-result-fanout/plan.md
**Data:** 2026-06-30

- [ ] `internal/domain/session.go` — `ResultPublisher.Publish(ctx, gameID, results)`
- [ ] `internal/publisher/log_publisher.go` — assinatura com gameID (fallback)
- [ ] `internal/service/session_service.go` — `ReportResult`: release-first + `Publish(gameID)`; fan-out best-effort
- [ ] `internal/service/session_service_test.go` — mock com gameID; release sempre; fan-out com gameID
- [ ] `internal/publisher/achievements_publisher.go` (+ httptest) — match_played (todos) + match_won (vencedores); X-Internal-Key; best-effort
- [ ] `cmd/server/main.go` — escolhe publisher (ACHIEVEMENTS_URL) + InternalKeyTransport; compose (ACHIEVEMENTS_URL, INTERNAL_API_KEY, depends_on)
- [ ] `go test ./... -race` + `go build ./...`; validar no Docker (loop: ReportResult -> achievements desbloqueia)
