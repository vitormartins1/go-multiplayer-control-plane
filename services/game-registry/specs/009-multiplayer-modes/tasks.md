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

## Follow-up 1 — SetModes ganha concorrência otimista (revisão do trade-off da decisão 1)

Achado no `/code-review` do projeto como um todo (2026-07-10): a decisão 1 do
`plan.md` ("last-write-wins... evita callback-under-tx/locks que o uso não
justifica") documentava conscientemente uma corrida no read-modify-write do
`SetMode`/`DeleteMode`. Ao investigar o achado original — que superestimava a
consequência ("17 modos, excedendo o cap") — confirmei que o `SetModes` faz um
`UPDATE` de substituição total (não um append), então o risco real não é
estourar o cap: é **lost update silencioso** (duas escritas concorrentes, a
segunda apaga a primeira sem aviso).

Perguntado ao usuário se ainda valia mudar um trade-off já documentado e
aceito na spec — optou por sinalizar o conflito (sem lock, sem retry
automático):

- `internal/domain/game.go`: `Game.MultiplayerModesVersion int64`; `GameRepository.SetModes` ganha `expectedVersion int64`.
- `internal/domain/errors.go`: `ErrModeConflict`.
- `internal/repository/postgres_profile.go`: `SetModes` faz CAS via `WHERE ... AND multiplayer_profile_version = $expected`, incrementando a versão no mesmo `UPDATE`; `RowsAffected() == 0` é ambíguo (sumiu vs. perdeu a corrida) — uma segunda leitura (`SELECT true WHERE id = $1 AND deleted_at IS NULL`) desambigua.
- `internal/repository/postgres_game.go`: `FindByID` também lê `multiplayer_profile_version`.
- `internal/service/profile_service.go`: `setMode`/`DeleteMode` passam `g.MultiplayerModesVersion` lido no mesmo `FindByID` que originou o read-modify-write.
- `internal/transport/http_handler.go`: `ErrModeConflict` → 409 `mode_conflict` (mesmo padrão de `ErrSlugTaken`/`ErrDefaultModeProtected`).
- `migrations/008_add_modes_version.sql`: `multiplayer_profile_version BIGINT NOT NULL DEFAULT 1`.
- Teste novo de integração: `TestPostgresGameRepository_SetModes_VersaoDesatualizada_ErrModeConflict` prova o CAS (duas escritas com a mesma versão base — a segunda falha com `ErrModeConflict`).

O trade-off da decisão 1 muda de "last-write-wins aceito" para "conflito
sinalizado, sem lock/retry automático" — o cliente (console admin) recebe 409
e decide recarregar. Barra verde: suíte completa (949 testes) + integração do
game-registry (25 testes, incl. o novo).
