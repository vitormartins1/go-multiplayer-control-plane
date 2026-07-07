# Tasks: 009-multiplayer-modes

- [x] T1 — domain: `MultiplayerMode`, `Game.MultiplayerModes`, erros novos, `GameService` (+SetMode/GetMode/ListModes/DeleteMode), `GameRepository.SetModes` (sai `UpsertMultiplayerProfile`)
- [x] T2 — repository: `storedMode`, `encodeModes`/`decodeModes` (v2 + upgrade lazy do v1, ordenação default-first), `SetModes`; `FindByID` popula modos + default
- [x] T3 — service: `validateProfile` extraída; `SetMode` (regex modeId, displayName ≤60, cap 16, upsert), `GetMode`, `ListModes`, `DeleteMode` (protege `default`); legado delega preservando displayName
- [x] T4 — transport: rotas da coleção, `modeDTO`/request body, `multiplayerModes` no `GET /games/{id}`, mapeamento dos 4 erros novos
- [x] T5 — testes service: casos da spec (upsert, regex, cap, delete/default, delegação legada sem regressão 006–008) — mock com `SetModes` (111 verdes, cover 89.4%)
- [x] T6 — testes repository (integração/testcontainers): round-trip v2 multi-modo; **upgrade lazy** de v1 gravado à mão; delete persiste (23 verdes)
- [x] T7 — testes transport: CRUD da coleção (200/204/400/404/409), summary no GET do jogo, alias legado intacto
- [x] T8 — barra verde: `go build ./...`, `go test ./... -race`, cover service 89.4%, `golangci-lint run` 0 issues
- [x] T9 — docs: estado M0 (lado registry) na tabela do `docs/game-models/README.md`; spec `Status: Aprovado`
