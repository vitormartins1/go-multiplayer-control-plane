# Tasks: 008-battle-royale-lobby

**Serviço:** matchmaking-service · **Spec:** [spec.md](./spec.md) · **Plan:** [plan.md](./plan.md)

Ordem: domínio → adapter de perfil → composição → testes → verificação. Session **não** é tocado. Marcar `[x]` conforme conclui.

## Domínio (ADR-0002 — zero deps externas)

- [x] **T1** — Em `internal/domain/matchmaking.go`: const `StructureSquads = "squads"` (junto de `StructureFFA`/`StructureTeams`), com comentário: teams "ragged" do `battle_royale` (último squad pode ter < `TeamSize`; `TeamCount` derivado por ⌈Size/TeamSize⌉ na composição).

## Adapter de perfil (registryclient)

- [x] **T2** — Em `internal/registryclient/client.go`: `profileResponse.Params` += `LobbySize int json:"lobbySize"`; const `archetypeBattleRoyale = "battle_royale"`.
- [x] **T3** — `client.go`: `Format` passa a aceitar `battle_royale` — `case archetypeBattleRoyale`: se `teamSize == 1` → `MatchFormat{Structure: StructureFFA, Size: lobbySize}`; se `teamSize >= 2` → `MatchFormat{Structure: StructureSquads, Size: lobbySize, TeamSize: teamSize}`. Manter o guard `ErrProfileUnavailable` para arquétipos ainda não suportados (`host_world_dropin`/`coop_mission`) e a checagem `Size <= 0` (agora também cobre `lobbySize <= 0`; adicionar checagem `teamSize <= 0` no ramo squad). `queue_arena` inalterado.

## Composição (matchmaker)

- [x] **T4** — Em `internal/service/matchmaker.go`: `composeTeams` += `case domain.StructureSquads` — particiona `group` em blocos de `format.TeamSize` via loop com `min(i+TeamSize, len(group))`, rótulos `fmt.Sprintf("squad-%d", k)`. Ragged por construção. `ffa`/`teams` intactos.

## Testes (mapeiam os "Casos de Teste Obrigatórios" da spec)

- [x] **T5** — `internal/registryclient/client_test.go`: `battle_royale` solo (`teamSize=1`) → `{ffa, Size=lobbySize}`; squad (`teamSize=4, lobbySize=60`) → `{squads, Size=60, TeamSize=4}`; `lobbySize<=0` e `teamSize<=0` → `ErrProfileUnavailable`. **Corrigir** `TestMatchSize_ArchetypeNaoSuportado_ErrProfileUnavailable`: trocar o body de `battle_royale` por `host_world_dropin` (arquétipo real não servido pelo `Format`).
- [x] **T6** — `internal/service/matchmaker_test.go`: `TestTryForm_Squads_QuatroPorSquad` (`lobbySize=60, teamSize=4` → 15 assignments `squad-1..squad-15`, 4 cada); `TestTryForm_Squads_Ragged` (`lobbySize=10, teamSize=4` → `squad-1`(4)/`squad-2`(4)/`squad-3`(2); soma = 10, nenhum jogador perdido); `TestTryForm_BattleRoyaleSolo_FFA` (Structure=ffa, Size=N → 1 assignment sem TeamID). Reusa `fakeProvider.formats` + mock provisioner do harness existente. Nomenclatura `TestTryForm_*`. `-race`.

## Verificação final

- [x] **T7 (automático)** — `go build ./...` ✅; `go test ./services/matchmaking-service/... -race -cover` (110 verdes; service+registryclient 93.2%, `Format`/`composeTeams` 100%) ✅; `golangci-lint run ./services/matchmaking-service/...` 0 issues ✅; `gofmt` limpo nos arquivos tocados ✅.
- [ ] **T8 (smoke manual — pendente do Vitor)** — subir docker-compose; cadastrar um jogo `battle_royale` (via BR0) e enfileirar `lobbySize` clientes de teste → conferir 1 lobby formado com os squads compostos e os connect tokens emitidos. (Opcional: com poucos clientes, usar um `lobbySize` pequeno no perfil.)
- [x] **T9** — Loop de aprendizado (CLAUDE.md): **nada novo de Go** — extensão de `switch` + partição de slice com `min` (ragged). Padrões já cobertos pelo `GO-LEARNINGS.md`. Nenhuma entrada nova (não inventar).
