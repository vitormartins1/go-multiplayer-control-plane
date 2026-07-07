# Tasks: 007-battle-royale-profile

**Serviço:** game-registry · **Spec:** [spec.md](./spec.md) · **Plan:** [plan.md](./plan.md)

Ordem hexagonal: domínio → repositório → serviço → transporte → verificação. Sem migration (D2). Marcar `[x]` conforme conclui.

## Domínio (ADR-0002 — zero deps externas)

- [x] **T1** — Em `internal/domain/game.go`: const `ArchetypeBattleRoyale = "battle_royale"` (junto das existentes) e consts de respawn (`RespawnOneLife = "one_life"`, `RespawnTeammateRevive = "teammate_revive"`, `RespawnArena1v1 = "arena_1v1"`, `RespawnRebootTimer = "reboot_timer"`).
- [x] **T2** — Em `game.go`: tipo `BattleRoyaleParams{ LobbySize int; TeamSize int; RespawnMode string; ScoringStats []string }` com doc-comment (ficha 03/ADR-0023). Adicionar `BattleRoyale *BattleRoyaleParams` ao `MultiplayerProfile` e atualizar o comentário do invariante (o ponteiro do `Archetype` declarado é o único não-nil) + o comentário do campo `Archetype`.

## Repositório (pgx) — sem migration

- [x] **T3** — Em `internal/repository/postgres_profile.go`: `storedBattleRoyale{ LobbySize int json:"lobbySize,omitempty"; TeamSize int json:"teamSize,omitempty"; RespawnMode string json:"respawnMode"; ScoringStats []string json:"scoringStats,omitempty" }`.
- [x] **T4** — `postgres_profile.go`: `case domain.ArchetypeBattleRoyale` em `encodeProfile` (nil-check do ponteiro → `perfil sem params`; marshal do `storedBattleRoyale`) e em `decodeProfile` (unmarshal → `p.BattleRoyale = &domain.BattleRoyaleParams{...}`).

## Serviço (regra de negócio)

- [x] **T5** — Em `internal/service/profile_service.go`: `supportedArchetypes` += `domain.ArchetypeBattleRoyale: true`; map de pacote `brRespawnModes = {one_life,teammate_revive,arena_1v1,reboot_timer → true}` (estilo `qaWinConditions`).
- [x] **T6** — `profile_service.go`: `validateBattleRoyale(p domain.BattleRoyaleParams) error` (D3): `teamSize ≥ 1`; `lobbySize ≥ 2` e `lobbySize ≥ 2*teamSize`; `respawnMode ∈ brRespawnModes`; cada `scoringStats` casa `scoringStatRE` (reusar a regex existente). Retorna `ErrInvalidProfileParams` com razão via `fmt.Errorf`.
- [x] **T7** — `profile_service.go`: `case domain.ArchetypeBattleRoyale` no `switch p.Archetype` de `SetMultiplayerProfile` — checa `p.BattleRoyale != nil` (senão `ErrInvalidProfileParams`) + `validateBattleRoyale`. Ordem geral (archetype→topology→params) inalterada.

## Transporte (HTTP)

- [x] **T8** — Em `internal/transport/http_handler.go`: `battleRoyaleParamsDTO{ LobbySize, TeamSize int; RespawnMode string; ScoringStats []string }` (JSON camelCase: `lobbySize`/`teamSize`/`respawnMode`/`scoringStats`).
- [x] **T9** — `http_handler.go`: `case domain.ArchetypeBattleRoyale` em `profileToDomain` (unmarshal do raw `params` → `domain.BattleRoyaleParams`) e em `profileToDTO` (marshal do `battleRoyaleParamsDTO` → raw). Espelha os cases existentes.

## Testes (mapeiam os "Casos de Teste Obrigatórios" da spec)

- [x] **T10** — `internal/service/profile_service_test.go`: `battle_royale` válido → upsert; `teamSize<1` → inválido; `lobbySize=6,teamSize=4` (viola `≥2×`) → inválido; `lobbySize=1` → inválido; cada `respawnMode` válido aceito + `respawnMode` inválido → inválido; `scoringStats` com item ruim → inválido, vazio → ok; `ErrGameNotFound` propagado; **regressão**: `queue_arena` e `host_world_dropin` válidos seguem OK. `TestSetMultiplayerProfile_BattleRoyale_*`. `-race -cover` (≥80%).
- [x] **T11** — `internal/repository/postgres_game_test.go` (testcontainers): upsert/`FindByID` round-trip do `battle_royale`; reconfigurar de `queue_arena` → `battle_royale` substitui e o ponteiro antigo fica nil.
- [x] **T12** — `internal/transport/http_handler_test.go` (`httptest`): `PUT .../multiplayer-profile` com `battle_royale` → 200 e 400 (params inválidos); `GET /games/{id}` inclui o `multiplayerProfile` `battle_royale` (round-trip completo DTO↔domínio↔DB no teste de integração, ou DTO↔domínio no unit).

## Verificação final

- [x] **T13 (automático)** — `go build ./...` ✅; `go test ./services/game-registry/... -race -cover` (unit: 103 verdes, service 87.6% cover; integração testcontainers: 6 verdes) ✅; `golangci-lint run` 0 issues ✅; `gofmt` limpo ✅.
- [ ] **T14 (smoke manual — pendente do Vitor)** — subir via docker-compose e validar com a collection `.http`: `PUT` perfil `battle_royale` → `GET /games/{id}` mostra o perfil → `GET /games` não mostra. Conferir que `queue_arena`/`host_world_dropin` seguem OK.
- [x] **T15** — Loop de aprendizado (CLAUDE.md): **nada novo de Go** nesta fatia — é aplicação do padrão *tagged union por ponteiro* já documentado em `docs/GO-LEARNINGS.md` (introduzido na spec 006). Nenhuma entrada nova (não inventar).
