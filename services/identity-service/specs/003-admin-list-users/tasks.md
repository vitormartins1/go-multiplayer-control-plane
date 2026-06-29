# Tasks: 003-admin-list-users
**Plan:** specs/003-admin-list-users/plan.md
**Data:** 2026-06-29

> Atravessa domain -> repository -> service -> transport. Sem migration. Admin sem auth.
> Marcar [x] conforme implementa.

## Fase 1 — Domain
- [ ] `internal/domain/account.go` — `AccountRepository` += `ListAccounts(ctx, limit, offset int) ([]Account, error)` e `CountAccounts(ctx) (int, error)`
- [ ] `internal/domain/account.go` — nova Port `PlayerLister { ListPlayers(ctx, limit, offset int) (players []Account, total int, err error) }`

## Fase 2 — Repository
- [ ] `internal/repository/postgres_account.go` — `ListAccounts` (`SELECT id, handle, created_at ... ORDER BY created_at DESC, id LIMIT $1 OFFSET $2`, sem `password_hash`) e `CountAccounts`
- [ ] `internal/repository/postgres_account_test.go` — integration (testcontainers): ordem por created_at desc; paginação (limit/offset); count global; `PasswordHash` vazio

## Fase 3 — Service
- [ ] `internal/service/admin_list.go` — `ListPlayers`: clamp (`limit` 1..200 default 50; `offset` >=0) + `ListAccounts` + `CountAccounts`
- [ ] `internal/service/admin_list_test.go` — unit (mock repo): clamp de limit (>200, <1, default); offset repassado; total

## Fase 4 — Transport
- [ ] `internal/transport/http_handler.go` — `NewHandler` += `players domain.PlayerLister`; rota `GET /admin/players`; parse `limit`/`offset`; DTO `{id, handle, createdAt}`; resposta `{total, players}`
- [ ] `internal/transport/http_handler_test.go` — 200 com lista+total; **sem `password_hash`** no corpo; query paginada; sem `Authorization` não bloqueia

## Fase 5 — Wire-up, docs e verificação
- [ ] `cmd/server/main.go` — `NewHandler(svc, issuer, svc)` (injeta o lister); ajustar setup dos handler tests existentes
- [ ] `api/identity.http` — exemplo do `GET /admin/players` (nota "interno, não expor")
- [ ] `go test ./... -race` verde; `go build ./...`
