# Spec: 008-battle-royale-lobby

**Serviço:** matchmaking-service
**Status:** Rascunho (aguardando aprovação)
**Data:** 2026-07-07
**ADR:** 0023 (perfil de multiplayer por jogo) — arquétipo `battle_royale`
**Roadmap:** `docs/game-models/battle_royale-roadmap.md` — fatia **BR1**
**Depende de:** BR0 (`game-registry/007` — declaração do `battle_royale`)

## Contexto

Segunda fatia do arquétipo `battle_royale` (ficha `docs/game-models/03-battle-royale.md`). O matchmaking já **lê o perfil do jogo** para derivar a formação do match (`MatchFormat` — specs `004`/`005`): `ffa` (Size=`playerCount`) ou `teams` (composição NvN posicional). Hoje o adapter `registryclient.Format` **rejeita** qualquer arquétipo != `queue_arena`.

Esta fatia ensina o matchmaking a formar **lobby de battle royale**: encher um **lobby grande** (`lobbySize` do perfil) e compor **squads** de `teamSize`. É uma **extensão em escala** do pipeline do `queue_arena` (roadmap, decisão #3) — reusa a fila FIFO, o agrupamento por `Size`, a composição posicional e a chamada `session.Provision(matchID, teams, gameID)`.

**Descoberta de escopo:** o `SessionProvisioner.Provision` é **agnóstico de arquétipo** — recebe `[]TeamAssignment` e emite os connect tokens com `team_id`. A diferença do `battle_royale` está no **resultado** (colocação ordinal = fatia BR2), **não** na formação. Portanto **esta fatia não toca o session-service**; é inteiramente dentro do matchmaking.

## Comportamento

### Derivar a formação do lobby do perfil `battle_royale`

- WHEN um jogador entra na fila (`FindMatch`) de um jogo cujo perfil é `battle_royale` → THE SYSTEM SHALL derivar o `MatchFormat` do perfil:
  - IF `teamSize == 1` (solo) → `MatchFormat{ Structure: "ffa", Size: lobbySize }` — cada jogador é independente (colocação por jogador; sem `team_id`).
  - IF `teamSize ≥ 2` (duo/squad) → `MatchFormat{ Structure: "squads", Size: lobbySize, TeamSize: teamSize }`.
- IF `lobbySize ≤ 0` ou `teamSize ≤ 0` (perfil corrompido) → **ErrProfileUnavailable** (o ticket não entra na fila — mesmo tratamento que o `queue_arena` dá a tamanho inválido).
- THE SYSTEM SHALL continuar derivando `queue_arena` (`ffa`/`teams`) **sem regressão**; arquétipos ainda não suportados (`host_world_dropin`, `coop_mission`) seguem **ErrProfileUnavailable** no `Format` (o `host_world_dropin` não usa fila — usa invasão/sinais, specs 006/007).

### Formar o lobby e compor squads

- WHEN a fila de um jogo `battle_royale` acumula `Size` (= `lobbySize`) tickets → THE SYSTEM SHALL formar o lobby (agrupamento FIFO por jogo, **inalterado** — reusa `tryForm`) e provisionar a sessão com a composição.
- WHEN a estrutura é `squads` → THE SYSTEM SHALL particionar o lobby em squads de `teamSize` na ordem FIFO, com rótulos estáveis `squad-1..squad-K`:
  - K = ⌈`lobbySize` / `teamSize`⌉ (arredonda para cima).
  - IF `lobbySize` **não** é múltiplo de `teamSize` → o **último squad** recebe o resto (`lobbySize mod teamSize` jogadores, squad menor). Nenhum ticket fica de fora.
- WHEN a estrutura é `ffa` (solo) → THE SYSTEM SHALL compor um único `TeamAssignment` sem `TeamID` com todos os jogadores (**reusa** o caminho FFA do `queue_arena`).
- THE SYSTEM SHALL manter a composição `teams` do `queue_arena` (blocos exatos `team-1..team-N`) **inalterada**.

### Modo (solo/duo/squad)

- THE SYSTEM SHALL derivar o modo do **`teamSize` do perfil do jogo** (um modo por jogo no MVP). Um jogo `battle_royale` é solo, duo **ou** squad conforme seu `teamSize`.
- Seleção de modo por requisição (filas solo/duo/squad simultâneas para o **mesmo** jogo) e **party** (grupos pré-formados entrando juntos num squad) ficam **fora do escopo** (follow-up) — o `FindMatch` **não** ganha campo novo.

## Contratos de Interface

### gRPC

Sem mudança no `.proto`: `FindMatch(game_id)` é a porta de entrada, como no `queue_arena`. O `game_id` já determina o perfil e, com ele, toda a formação.

### Domínio — extensão

- Nova constante `StructureSquads = "squads"` (em `matchmaking.go`, junto de `StructureFFA`/`StructureTeams`).
- `MatchFormat` **inalterado** em forma (`Size`, `Structure`, `TeamCount`, `TeamSize`); `TeamSize` passa a valer também para `squads` (`TeamCount` fica 0 em squads — a contagem é derivada por ⌈Size/TeamSize⌉ na composição).

### Adapter `registryclient.Format` — extensão

- `profileResponse.Params` ganha `LobbySize int` e reusa o `TeamSize int` já existente.
- Novo `case archetypeBattleRoyale` na derivação; `archetypeBattleRoyale = "battle_royale"`.

### `composeTeams` (matchmaker) — extensão

- Novo ramo para `StructureSquads`: particiona em ⌈Size/TeamSize⌉ blocos (último ragged), rótulos `squad-K`.

## Modelo de Dados

Nenhum — matchmaking é stateless (fila em memória). Sem migration, sem store.

## Casos de Teste Obrigatórios

- [ ] (registryclient) perfil `battle_royale` `teamSize=1` → `MatchFormat{ffa, Size=lobbySize}`
- [ ] (registryclient) perfil `battle_royale` `teamSize=4, lobbySize=60` → `MatchFormat{squads, Size=60, TeamSize=4}`
- [ ] (registryclient) `battle_royale` com `lobbySize<=0` ou `teamSize<=0` → `ErrProfileUnavailable`
- [ ] (registryclient) `queue_arena` ffa/teams seguem derivando certo (sem regressão); `host_world_dropin` → `ErrProfileUnavailable` no `Format`
- [ ] (composeTeams) `squads` `lobbySize=60, teamSize=4` → 15 squads de 4, rótulos `squad-1..squad-15`
- [ ] (composeTeams) `squads` **ragged** `lobbySize=10, teamSize=4` → `squad-1`(4), `squad-2`(4), `squad-3`(2); nenhum jogador perdido
- [ ] (composeTeams) `squads` `teamSize=1`? (n/a — solo vira `ffa` no `Format`; garantir que solo produz FFA de 1 assignment)
- [ ] (composeTeams) `ffa` (solo BR) → 1 assignment sem TeamID com todos; `teams` (queue_arena) **inalterado**
- [ ] (matchmaker, integração leve) fila de um jogo `battle_royale` squad enche `lobbySize` tickets → forma 1 lobby, provisiona com os squads compostos (mock do provisioner) — reusa o harness de teste do matchmaker

## Fora do escopo

- **Resultado por colocação** (`placement`) — fatia BR2 (session).
- **Modo por requisição** (solo/duo/squad na mesma fila do jogo) e **party** — follow-up.
- **Skill/SBMM e região** no lobby grande — follow-up (mesmo do `queue_arena`).
- **Lobby timeout / bot-fill** (formar com menos que `lobbySize` após espera) — follow-up; MVP espera o lobby cheio, como o `queue_arena` espera `Size` exato.
- **session-service** — não é tocado (a formação já é agnóstica de arquétipo).
