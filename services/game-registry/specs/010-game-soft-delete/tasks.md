# Tasks: 010-game-soft-delete

- [x] T1 — migration `006_add_deleted_at.sql` (`ALTER TABLE games ADD COLUMN deleted_at TIMESTAMPTZ`)
- [x] T2 — domain: `GameService.Delete` + `GameRepository.SoftDelete` (doc dos Ports: deletado = inexistente)
- [x] T3 — repository (games): `SoftDelete` (UPDATE condicional, RowsAffected 0 → `ErrGameNotFound`); filtro `deleted_at IS NULL` em `List`/`FindByID`/`UpdateName`/`SetModes`
- [x] T4 — repository (presença): gate de jogo vivo no `Upsert` (INSERT…SELECT WHERE EXISTS); filtro em `GamesByPlayer` e `ListByGame`
- [x] T5 — service: `Delete` repassando ao repo
- [x] T6 — transport: rota `DELETE /games/{id}` → 204/404
- [x] T7 — testes service: delete vivo/repetido/inexistente; pós-delete Get/Rename/SetMode → `ErrGameNotFound`; mock com `SoftDelete`
- [x] T8 — testes repository (integração/testcontainers): soft delete preserva linha e some das leituras; slug reservado (`ErrSlugTaken`); presença (Upsert 404, reversa omite, ListByGame vazio)
- [x] T9 — testes transport: DELETE 204/404 + internal key; `GET /games/{id}` de deletado → 404
- [x] T10 — barra verde: `go build ./...`, `go test ./... -race` (cover service ≥ 80%), `golangci-lint run` 0 issues; recriar volume do `game-registry-postgres` e validar migration no compose
- [x] T11 — docs: spec `Status: Aprovado`; `docs/NEXT-STEPS.md` (backlog item 5 → feito)
