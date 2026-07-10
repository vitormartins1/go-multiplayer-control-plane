# Tasks 012 — campo `Fleet` por jogo

- [ ] T1. Migration `ALTER TABLE games ADD COLUMN fleet TEXT` + mount no `docker-compose.yml` + aplicar no volume local existente
- [ ] T2. `domain.Game.Fleet` + `domain.ErrInvalidFleet` + assinaturas novas/alteradas em `GameService`/`GameRepository`
- [ ] T3. Repository: `UpdateFleet`, `selectCols`/`scanGame`/`FindByID` com a coluna nova
- [ ] T4. Service: `SetFleet` (validação `fleetRE`) + `GetMode` devolvendo `(*MultiplayerMode, string, error)`
- [ ] T5. Transport: rota `PUT /games/{id}/fleet`, `gameDTO.Fleet`, `modeDTO.Fleet`, `toModeDTO` com fleet, `writeDomainError` para `ErrInvalidFleet`
- [ ] T6. Testes: service (4 casos), repository (round-trip), transport (rota nova + fleet embutido no getMode); mocks atualizados
- [ ] T7. `go build ./... && go test ./... -race -cover` verde no game-registry
