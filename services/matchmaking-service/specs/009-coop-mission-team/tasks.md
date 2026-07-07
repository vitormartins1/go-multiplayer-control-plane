# Tasks: 009-coop-mission-team

**Serviço:** matchmaking-service · **Spec:** [spec.md](./spec.md) · **Plan:** [plan.md](./plan.md)

Ordem: adapter → testes → verificação. Sem mudança de domínio/session/migration.

## Adapter (registryclient)

- [x] **T1** — Em `internal/registryclient/client.go`: const `archetypeCoopMission = "coop_mission"` (junto de `archetypeQueueArena`/`archetypeBattleRoyale`).
- [x] **T2** — `client.go`: `case archetypeCoopMission` no `switch p.Archetype` do `Format` — valida `p.Params.TeamSize >= 2` (senão `ErrProfileUnavailable` com razão); monta `domain.MatchFormat{Structure: domain.StructureTeams, TeamCount: 1, TeamSize: p.Params.TeamSize, Size: p.Params.TeamSize}`. Atualizar o doc-comment do `Format` (passa a servir também `coop_mission`).

## Testes

- [x] **T3** — `internal/registryclient/client_test.go`: `TestMatchSize_CoopMission_TimeUnico` (`teamSize=4` → `{teams, TeamCount:1, TeamSize:4, Size:4}`); `TestMatchSize_CoopMission_TeamSizeInvalido_ErrProfileUnavailable` (`teamSize=1`).
- [x] **T4** — `internal/service/matchmaker_test.go`: fila `coop_mission` (format `teams`/`TeamCount:1`/`Size:teamSize`) enche `teamSize` tickets → 1 grupo formado, `Provision` recebe 1 assignment `team-1` com os N jogadores. Reusa o harness existente (mock do provisioner). Se um teste `teams` já cobrir `composeTeams` com `TeamCount=1`, adicionar caso explícito.

## Verificação final

- [x] **T5 (automático)** — `go build ./...`; `go test ./services/matchmaking-service/... -race -cover` (≥80% service); `golangci-lint run ./services/matchmaking-service/...` 0 issues; `gofmt` limpo nos arquivos tocados.
- [x] **T6** — Loop de aprendizado (CLAUDE.md): **nada novo de Go** esperado (extensão de `switch` por arquétipo já documentada). Confirmar e não inventar.
