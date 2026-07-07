# Tasks: 009-participation-consumer

Fatia H2 (sink achievements) do `host_world_dropin` (ADR-0023). Consumer despacha por `kind`; reage a `participation_event` por papel, idempotente por `(sessionID, playerID, event)`. Ver `plan.md` (D1–D5). Marcar `[x]` ao concluir.

Baseline: `go test ./services/achievements-service/... -race` **verde** antes.

## Domínio

- [x] **T1 — ParticipationEventTypes + Port.** `internal/domain/achievement.go`: consts `EventInvasionWon`/`EventCoopBossHelped`/`EventWorldJoined`; função pura `ParticipationEventTypes(role, event, outcome string) []string` (mapa da spec; sem tradução → nil). Método `ApplyParticipationEvent(ctx, gameID, sessionID, playerID, event string, eventTypes []string, at time.Time) (bool, error)` na Port `AchievementRepository`. `internal/domain/participation_test.go`: tabela.

## Repositório

- [x] **T2 — migration + repo.** `migrations/006_processed_participations.sql`: tabela PK `(session_id, player_id, event)` (espelha `004`). `internal/repository/postgres_achievement.go`: `ApplyParticipationEvent` — tx única: marca `processed_participations` (`ON CONFLICT DO NOTHING`; `RowsAffected()==0` → `dup=true`, commit) senão `findByEventType`+`recordProgress` por event type; commit. Teste integração (`//go:build integration`): reprocessa → dup sem dobrar; distintos acumulam; event type sem conquista → processado sem progresso.

## Serviço

- [x] **T3 — ApplyParticipationEvent.** `internal/service/achievement_service.go`: método concreto que chama `domain.ParticipationEventTypes` e delega ao repo. `achievement_service_test.go`: mock repo += `ApplyParticipationEvent`; testes (tradução com event types certos; `LEFT`/`DIED` → lista vazia; `dup` propaga).

## Consumer

- [x] **T4 — dispatch por kind.** `cmd/consumer/main.go` `makeHandler`: decodifica `{kind}` e ramifica — `win_loss_draw`/ausente-com-matchId → `ApplyMatchResult`; `participation_event` → decodifica campos e `ApplyParticipationEvent`; desconhecido → log debug + descarta (`false, nil`). Teste do handler se houver harness.

## Fechamento

- [x] **T5 — verde + lint + docs.** `go test ./services/achievements-service/... -race -cover` (≥80% service/domain). `golangci-lint run`. Caminho `win_loss_draw` sem regressão. Atualizar a tabela de estado do README de game-models (H2 sink achievements).
