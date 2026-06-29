# Tasks: 004-game-scope
**Plan:** specs/004-game-scope/plan.md
**Data:** 2026-06-29

> game_id como dimensão (catálogo + progresso). Header X-Game-Id (data plane),
> path {gameId} (admin). Jogo default `sandbox`. Marcar [x] conforme implementa.

## Fase 1 — Companion (game-registry seed sandbox)
- [ ] `services/game-registry/migrations/002_seed_sandbox.sql` — INSERT do jogo `sandbox` (id fixo) ON CONFLICT DO NOTHING
- [ ] `infra/docker/docker-compose.yml` — montar a 002 no `game-registry-postgres`

## Fase 2 — Migração (achievements)
- [ ] `migrations/003_add_game_scope.sql` — `game_id` nas duas tabelas; PK `(game_id, id)` e `(game_id, player_id, achievement_id)`; FK composta; backfill `sandbox`; DROP DEFAULT pós-backfill

## Fase 3 — Domain
- [ ] `internal/domain/achievement.go` — `Achievement` += `GameID`; Ports `AchievementService`/`AchievementRepository` ganham `gameID`; novo `CreateAchievement`
- [ ] `internal/domain/errors.go` — `ErrAchievementExists`, `ErrInvalidAchievement`

## Fase 4 — Repository
- [ ] `internal/repository/postgres_achievement.go` — `game_id` em todas as queries; `RecordProgress` ON CONFLICT `(game_id, player_id, achievement_id)`; novo `CreateAchievement` (`23505` -> `ErrAchievementExists`)
- [ ] `internal/repository/postgres_achievement_test.go` — integration: game_id em tudo; isolamento entre jogos; mesmo id em jogos diferentes coexiste

## Fase 5 — Service
- [ ] `internal/service/achievement_service.go` — repassar `gameID`; `CreateAchievement` (valida slug/name/target)
- [ ] `internal/service/achievement_service_test.go` — atualizar chamadas com `gameID`; testes do create (ok/inválido/existe)

## Fase 6 — Transport
- [ ] `internal/transport/http_handler.go` — helper `gameID(r)` do header `X-Game-Id` (vazio -> 400); rotas de jogador passam o gameID; rotas admin viram `/admin/games/{gameId}/...` (create+list+player); remove rotas admin globais da 003
- [ ] `internal/transport/http_handler_test.go` — header X-Game-Id nos testes de jogador; novas rotas admin; create 201/400/409; data plane sem header -> 400

## Fase 7 — Docs e verificação
- [ ] `api/achievements.http` — header X-Game-Id + rotas admin por jogo
- [ ] `go test ./... -race` + `go test -tags=integration ./services/achievements-service/internal/repository/`; `go build ./...`
- [ ] Docker (`down -v` para aplicar a migração): isolamento entre jogos; criar achievement via rota admin; eventos por jogo; dados antigos -> sandbox
