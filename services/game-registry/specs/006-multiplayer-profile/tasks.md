# Tasks: 006-multiplayer-profile

**Serviço:** game-registry · **Spec:** [spec.md](./spec.md) · **Plan:** [plan.md](./plan.md)

Ordem hexagonal: domínio → migration → repositório → serviço → transporte → verificação. Marcar `[x]` conforme conclui.

## Domínio (ADR-0002 — zero deps externas)

- [x] **T1** — Em `internal/domain/game.go`: tipos `QueueArenaParams` (campos da spec: `Structure`, `PlayerCount`, `TeamCount`, `TeamSize`, `WinCondition`, `ScoreTarget`, `DurationSeconds`, `MatchStructure`, `RoundsToWin`, `AllowDraw`, `ScoringStats []string`) e `MultiplayerProfile{ Archetype, Topology string; QueueArena *QueueArenaParams }` (D2). Comentário do invariante (ponteiro do arquétipo declarado é o único não-nil).
- [x] **T2** — Em `game.go`: adicionar `MultiplayerProfile *MultiplayerProfile` ao struct `Game` (nil = sem perfil). Estender a Port `GameService` com `SetMultiplayerProfile(ctx, gameID string, p MultiplayerProfile) (*MultiplayerProfile, error)` e `GetMultiplayerProfile(ctx, gameID string) (*MultiplayerProfile, error)`; a Port `GameRepository` com `UpsertMultiplayerProfile(ctx, gameID string, p MultiplayerProfile) error`.
- [x] **T3** — Em `internal/domain/errors.go`: `ErrUnsupportedArchetype`, `ErrUnsupportedTopology`, `ErrInvalidProfileParams`, `ErrProfileNotFound`.

## Migration

- [x] **T4** — `migrations/005_add_multiplayer_profile.sql`: `ALTER TABLE games ADD COLUMN multiplayer_profile JSONB;` (nullable, sem backfill — comentário citando ADR-0023/spec 006).

## Repositório (pgx)

- [x] **T5** — Em `internal/repository/postgres_game.go`: `UpsertMultiplayerProfile` = `UPDATE games SET multiplayer_profile = $2 WHERE id = $1`; serializa `MultiplayerProfile` → JSON (`{archetype, topology, params}`); `RowsAffected() == 0` → `domain.ErrGameNotFound` (padrão do `UpdateName`).
- [x] **T6** — Em `postgres_game.go`: `FindByID` passa a ler a coluna `multiplayer_profile` e popular `Game.MultiplayerProfile` (decode JSON → struct; NULL → nil). `List` **inalterado** (não lê o perfil).

## Serviço (regra de negócio)

- [x] **T7** — `internal/service/profile_service.go`: enums (`archétipos`/`topologias`/`winCondition`/`matchStructure`/`structure` suportados) e `scoringStatRE = ^[a-z][a-z0-9_]*$` como `var`/`const` de pacote (estilo `slugRE`).
- [x] **T8** — `profile_service.go`: `validateQueueArena(p QueueArenaParams) error` com as regras condicionais da spec (ffa→`PlayerCount≥2`; teams→`TeamCount≥2`&`TeamSize≥1`; `first_to_score`→`ScoreTarget>0`; `highest_at_time`→`DurationSeconds>0`; `best_of_rounds`→`RoundsToWin>0`; cada `ScoringStats` casa a regex; lista vazia ok) → `ErrInvalidProfileParams`.
- [x] **T9** — `profile_service.go`: `SetMultiplayerProfile` no `*service` — valida `archetype`→`topology`→`QueueArena` (não-nil + `validateQueueArena`), depois `repo.UpsertMultiplayerProfile`; erros wrappados (`fmt.Errorf("game-registry service: set profile: %w", err)`). Devolve o perfil persistido.
- [x] **T10** — `profile_service.go`: `GetMultiplayerProfile` no `*service` — reusa `repo.FindByID`: ausente → `ErrGameNotFound`; presente com `MultiplayerProfile == nil` → `ErrProfileNotFound`; senão devolve o perfil (D3).

## Transporte (HTTP)

- [x] **T11** — Em `internal/transport/http_handler.go`: DTOs `profileDTO{Archetype, Topology, Params queueArenaParamsDTO}` + `queueArenaParamsDTO` (campos JSON camelCase da spec); mapeadores DTO↔domínio.
- [x] **T12** — `http_handler.go`: handler `setProfile` (`PUT /games/{id}/multiplayer-profile`) — decode (`DisallowUnknownFields`) → `SetMultiplayerProfile` → 200 DTO | erro; handler `getProfile` (`GET /games/{id}/multiplayer-profile`) → `GetMultiplayerProfile` → 200 | 404. Registrar as 2 rotas em `Register`.
- [x] **T13** — `http_handler.go`: `get` (de `GET /games/{id}`) passa a devolver DTO estendido com `multiplayerProfile` (perfil ou `null`, **sem** `omitempty`); `list` e `toDTO` da lista **inalterados**.
- [x] **T14** — `http_handler.go`: `writeDomainError` += `ErrUnsupportedArchetype`→400, `ErrUnsupportedTopology`→400, `ErrInvalidProfileParams`→400, `ErrProfileNotFound`→404 (códigos estáveis: `unsupported_archetype`, `unsupported_topology`, `invalid_profile_params`, `profile_not_found`).

## Testes (mapeiam os "Casos de Teste Obrigatórios" da spec)

- [x] **T15** — `internal/service/profile_service_test.go` (`testify/mock` do repo): upsert válido; `ErrUnsupportedArchetype`/`ErrUnsupportedTopology`; cada regra condicional inválida; `scoringStats` bom/ruim (vazio ok); `ErrGameNotFound` propagado. Nomenclatura `TestSetMultiplayerProfile_Cenario_Resultado`. Rodar `-race -cover` (≥80%).
- [x] **T16** — `internal/repository/postgres_game_test.go` (testcontainers): upsert round-trip; reconfiguração substitui; `FindByID` popula o perfil; jogo sem perfil → nil; upsert em id inexistente → `ErrGameNotFound`.
- [x] **T17** — `internal/transport/http_handler_test.go` (`httptest`): `PUT` 200/400/404; `GET .../multiplayer-profile` 200/404; `GET /games/{id}` inclui `multiplayerProfile` (perfil e `null`); `GET /games` **não** inclui.

## Verificação final

- [x] **T18 (automático)** — `go build`, `go test -race -cover` (unit: service 81.5%, transport 87.2%; integração testcontainers ✅ aplica a migration 005 contra Postgres real), `golangci-lint` (0 issues), `gofmt` ✅.
- [ ] **T18 (smoke manual — pendente do Vitor)** — subir via docker-compose e validar o fluxo com a collection `.http` (PUT perfil → GET jogo mostra o perfil → GET lista não mostra) + conferir métricas RED das rotas novas no Grafana.
- [x] **T19** — Fechar o loop de aprendizado (CLAUDE.md): registrar em `docs/GO-LEARNINGS.md` o padrão **tagged union por ponteiro** (D2) se ainda não houver entrada equivalente — exemplo curto + referência ao código real (`game-registry/internal/domain/game.go`).
