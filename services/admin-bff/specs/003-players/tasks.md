# Tasks: 003-players
**Plan:** specs/003-players/plan.md
**Data:** 2026-06-29

> Proxy fino do GET /admin/players do identity. Mesmo padrão do adapter de achievements.
> Marcar [x] conforme implementa.

## Fase 1 — Domain
- [x] `internal/domain/admin.go` — `Player{ID,Handle,CreatedAt}`, `PlayerPage{Total,Players}`; Port `PlayerDirectory`
- [x] `internal/domain/errors.go` — `ErrIdentityUnavailable`

## Fase 2 — Identity client (adapter)
- [x] `internal/identity/client.go` — `GET {IDENTITY_URL}/admin/players?limit=&offset=`; mapeia `{total,players[]}` -> `PlayerPage`; fora -> `ErrIdentityUnavailable`
- [x] `internal/identity/client_test.go` — `httptest`: mapeia página; repassa query; serviço fora -> erro

## Fase 3 — Service
- [x] `internal/service/admin_service.go` — `New` ganha `PlayerDirectory`; método `Players(ctx, limit, offset)` (delega)
- [x] `internal/service/admin_service_test.go` — delega + repassa limit/offset (fake directory); ajustar `New(...)` nos testes existentes

## Fase 4 — Transport
- [x] `internal/transport/http_handler.go` — `AdminService` += `Players`; rota `GET /api/players` (parse limit/offset; `ErrIdentityUnavailable`->502)
- [x] `internal/transport/http_handler_test.go` — 200 (página) + 502 (identity fora); ajustar fakeService

## Fase 5 — Wire-up, infra e verificação
- [x] `cmd/server/main.go` — lê `IDENTITY_URL`; cria o identity client (otelhttp); injeta no `service.New`
- [x] `api/README.md` + `README.md` — `/api/players`
- [x] `infra/docker/docker-compose.yml` — `IDENTITY_URL` + `depends_on: identity-service`
- [x] `go test ./... -race` + `go build ./...`; validar contra o identity real
