# Plan: 008-battle-royale-lobby

**Serviço:** matchmaking-service · **Spec:** [spec.md](./spec.md) · **ADR:** 0023 · **Roadmap:** BR1

## Objetivo

Ensinar o matchmaking a formar **lobby de battle royale** (encher `lobbySize`, compor squads), reusando o pipeline do `queue_arena` (fila FIFO, `tryForm` por `Size`, composição posicional, `session.Provision`). Mudança **cirúrgica e aditiva**, confinada ao matchmaking (session **não** é tocado — a formação já é agnóstica de arquétipo). Sem `.proto`, sem migration, sem store.

## Decisões técnicas

### D1 — `Format` (registryclient) ganha o `case battle_royale`; solo→ffa, squad→squads

- `profileResponse.Params` ganha `LobbySize int` (reusa o `TeamSize` já presente). Novo `archetypeBattleRoyale = "battle_royale"`.
- Derivação:
  - `teamSize == 1` → `MatchFormat{Structure: StructureFFA, Size: lobbySize}` (solo BR = FFA; colocação por jogador, sem `team_id` — reusa 100% o caminho FFA).
  - `teamSize >= 2` → `MatchFormat{Structure: StructureSquads, Size: lobbySize, TeamSize: teamSize}` (`TeamCount` fica 0 — a contagem é ⌈Size/TeamSize⌉, derivada na composição).
  - `lobbySize <= 0 || teamSize <= 0` → `ErrProfileUnavailable` (mesmo tratamento do tamanho inválido do `queue_arena`).
- **Por quê solo→ffa e não squads com TeamSize=1:** solo reusa exatamente a composição FFA existente (um assignment, sem `team_id`), sem caso especial. Squads é só para `teamSize >= 2`.

### D2 — `StructureSquads` = teams ragged; `composeTeams` ganha o ramo

- Nova const `domain.StructureSquads = "squads"`. Diferença do `teams` (queue_arena): o `teams` garante `Size = TeamCount*TeamSize` (blocos exatos); o `squads` permite **resto** (BR0 aceita `lobbySize` não-múltiplo de `teamSize`).
- `composeTeams` (matchmaker.go): novo ramo `case StructureSquads` — itera em passos de `TeamSize`, cada bloco `group[i:min(i+TeamSize, len)]`, rótulos `squad-1..squad-K` (`K = ⌈len/TeamSize⌉`). O último bloco pode ter < `TeamSize` (ragged). Loop robusto a ragged por construção (`min`).
- `teams` e `ffa` ficam **intactos** (ramos separados). Não unifico `teams`+`squads` para não arriscar o `queue_arena` (que depende dos rótulos `team-N` e de blocos exatos).

### D3 — `tryForm` inalterado

- `tryForm` agrupa `ts[0].format.Size` tickets por jogo. Para BR, `Size = lobbySize`. Nenhuma mudança: o lobby grande é só um `Size` maior. A composição (squads vs teams vs ffa) é decidida depois, em `composeTeams`, a partir do `Structure` do format.

### D4 — Modo derivado do perfil (um modo por jogo)

- O `teamSize` do perfil determina solo/duo/squad. Sem campo novo no ticket/`FindMatch`. Filas simultâneas por modo e party = follow-up (spec, Fora do escopo).

## Mudanças por camada (arquivos)

| Arquivo | Mudança |
|---|---|
| `internal/domain/matchmaking.go` | const `StructureSquads = "squads"` (+ comentário: teams ragged do battle_royale) |
| `internal/registryclient/client.go` | `profileResponse.Params` += `LobbySize`; const `archetypeBattleRoyale`; `Format` += `case battle_royale` (solo→ffa, squad→squads); mantém o guard de arquétipo não suportado p/ os demais |
| `internal/service/matchmaker.go` | `composeTeams` += `case StructureSquads` (partição ragged, rótulos `squad-K`) |
| **session-service** | **sem mudança** (formação agnóstica de arquétipo) |
| `.proto` / migrations / store | **sem mudança** |

## Testes (mapeiam os casos da spec)

- **registryclient** (`client_test.go`, `httptest`): `battle_royale` solo→ffa; squad `60/4`→`squads,Size=60,TeamSize=4`; `lobbySize<=0`/`teamSize<=0`→`ErrProfileUnavailable`; **regressão**: `queue_arena` ffa/teams ok. **Corrigir** `TestMatchSize_ArchetypeNaoSuportado_ErrProfileUnavailable` (hoje usa `battle_royale` como não-suportado) → trocar por `host_world_dropin` (arquétipo real que o `Format` não serve — usa invasão/sinais).
- **matchmaker** (`matchmaker_test.go`, `fakeProvider`): `TestTryForm_Squads_*` — `60/4` → 15 squads `squad-1..15`; **ragged** `10/4` → `squad-1`(4)/`squad-2`(4)/`squad-3`(2), nenhum jogador perdido; solo BR (`ffa`, Size=N) → 1 assignment sem TeamID. Reusa o harness `fakeProvider`/`mock provisioner`.
- Cobertura ≥ 80% em `service`/`domain` (regra do projeto). `-race`.

## Observabilidade

Sem métrica nova: o lobby BR usa o mesmo caminho `FindMatch → tryForm → provision` já instrumentado (fila/matches ok/falha — specs 001/010). Distinção por arquétipo no funil = follow-up (não bloqueia).

## Riscos / fora do escopo

- **Risco baixo:** aditivo, ramos separados; `queue_arena` intacto (garantido pelos testes de regressão). O ponto de atenção é o teste antigo que usa `battle_royale` como não-suportado — corrigido explicitamente.
- Colocação (BR2), modo por requisição/party, SBMM/região, timeout/bot-fill — fora daqui.
