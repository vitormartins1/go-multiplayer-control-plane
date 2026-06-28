# Tasks: 002-partial-progress
**Plan:** specs/002-partial-progress/plan.md
**Data:** 2026-06-28

> Evolução da 001. Ordem: migration → domain → service → repository → transport → infra/validação.
> Testes acompanham cada camada. Marcar [x] conforme implementa.

## Fase 1 — Migration e schema
- [x] `migrations/002_add_progress.sql` — `ALTER achievements ADD target INT NOT NULL DEFAULT 1`; `ALTER player_achievements ADD progress INT NOT NULL DEFAULT 0`, `ALTER unlocked_at DROP NOT NULL`; seed `ten_wins` (event_type `match_won`, target 10)
- [x] `infra/docker/docker-compose.yml` — montar `002_add_progress.sql` no initdb do `achievements-postgres`

## Fase 2 — Domain
- [x] `internal/domain/achievement.go` — `Achievement` +`Target int`; `PlayerAchievement` +`Progress int`; novo `PlayerProgress{AchievementID, Progress, UnlockedAt}`; Ports: `Unlock`→`RecordProgress(ctx, playerID, achievementID, target, at) (justUnlocked bool, err)`, `ListPlayerUnlocks`→`ListPlayerProgress`; remover struct `Unlock`

## Fase 3 — Service
- [x] `internal/service/achievement_service.go` — `RecordEvent` chama `RecordProgress(a.Target)` e coleta as `justUnlocked`; `ListForPlayer` mapeia progresso (clampa `Progress` ao `Target`, `Unlocked` = `unlockedAt` não-zero)
- [x] `internal/service/achievement_service_test.go` — atualizar mock; casos: incrementa sem desbloquear (target>1); atinge a meta → desbloqueia; pós-desbloqueio → no-op; target=1 → desbloqueia no 1º; `ListForPlayer` reflete progress/target

## Fase 4 — Repository
- [x] `internal/repository/postgres_achievement.go` — `RecordProgress` (UPSERT atômico do plan, retorno `unlocked_at` como `*time.Time`); `ListPlayerProgress`; `ListCatalog`/`FindByEventType` passam a ler `target`
- [x] `internal/repository/postgres_achievement_test.go` — `WithInitScripts` 001+002; casos: progride sem desbloquear; atinge meta → `justUnlocked=true`; evento extra pós-desbloqueio → `false` e estado imutável; target=1 → desbloqueia no 1º; progresso isolado por jogador

## Fase 5 — Transport
- [x] `internal/transport/http_handler.go` — `achievementDTO` +`target`; `playerAchievementDTO` +`progress`/`target`
- [x] `internal/transport/http_handler_test.go` — atualizar fakes e asserts (progress/target na resposta)

## Fase 6 — Verificação e infra
- [x] `go test ./... -race -cover` (unit) e `-tags=integration` (repository) verdes; `go build ./...`
- [x] Validar no Docker: `match_won` 1..10× → `ten_wins` progride até desbloquear (10/10); `first_win` desbloqueia no 1º; `GET /me/achievements` mostra progress/target
- [x] `services/achievements-service/api/achievements.http` — adicionar exemplo de progresso parcial
