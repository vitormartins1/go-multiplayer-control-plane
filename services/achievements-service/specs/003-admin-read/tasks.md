# Tasks: 003-admin-read
**Plan:** specs/003-admin-read/plan.md
**Data:** 2026-06-29

> Feature só de transport: reusa `ListForPlayer`/`ListCatalog` (001/002). Rotas `/admin/...` sem auth.
> Marcar [x] conforme implementa.

## Fase 1 — Transport (rotas admin)
- [x] `internal/transport/http_handler.go` — `Register`: adicionar `GET /admin/players/{playerID}/achievements` e `GET /admin/achievements`, **fora** do wrapper `h.auth`
- [x] `internal/transport/http_handler.go` — `adminListForPlayer`: lê `r.PathValue("playerID")` e chama `h.svc.ListForPlayer`; reusa `toPlayerAchievementDTOs`
- [x] `internal/transport/http_handler.go` — `adminListCatalog`: chama `h.svc.ListCatalog`; reusa `toAchievementDTOs`

## Fase 2 — Testes
- [x] `internal/transport/http_handler_test.go` — `GET /admin/players/{id}/achievements` retorna o catálogo anotado para o `id` do **path** (mock do service)
- [x] `internal/transport/http_handler_test.go` — jogador sem progresso -> `progress=0`, `unlocked=false`
- [x] `internal/transport/http_handler_test.go` — `GET /admin/achievements` retorna o catálogo
- [x] `internal/transport/http_handler_test.go` — endpoint admin **sem** `Authorization` responde 200 (não bloqueia, ao contrário do `/me/achievements`)

## Fase 3 — Docs e verificação
- [x] `services/achievements-service/api/` (`.http` ou README) — exemplos dos endpoints `/admin/...` + nota "interno, não expor publicamente"
- [x] `go test ./... -race` verde; `go build ./...`
