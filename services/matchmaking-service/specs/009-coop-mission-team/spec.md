# Spec: 009-coop-mission-team

**Serviço:** matchmaking-service
**Status:** Aprovado (batch C1–C4)
**Data:** 2026-07-07
**ADR:** 0023 (perfil de multiplayer por jogo) — arquétipo `coop_mission`
**Roadmap:** `docs/game-models/coop_mission-roadmap.md` — fatia **C1**
**Depende de:** C0 (`game-registry/008` — declaração do `coop_mission`)

## Contexto

Segunda fatia do arquétipo `coop_mission` (ficha `docs/game-models/04-coop-pve-mission.md`). O matchmaking já **lê o perfil do jogo** para derivar a formação (`MatchFormat` — specs `004`/`005`/`008`): `ffa`, `teams` (NvN) e `squads` (battle_royale). Hoje o adapter `registryclient.Format` rejeita `coop_mission` com `ErrProfileUnavailable` (arquétipo não suportado).

Esta fatia ensina o matchmaking a formar o **time co-op**: um **único time** de `teamSize` jogadores. Pela ficha, co-op é "1 time contra o ambiente" — o caso **degenerado** do NvN do `queue_arena` (roadmap, decisão #3): **um** lado, todos no mesmo `team_id`.

**Descoberta de escopo (herdada da BR1):** o `SessionProvisioner.Provision` é **agnóstico de arquétipo** — recebe `[]TeamAssignment` e emite os connect tokens com `team_id`. A diferença do `coop_mission` está no **resultado** (coletivo = fatia C2), **não** na formação. Portanto **esta fatia não toca o session-service**.

## Decisão de modelagem (reuso máximo)

Co-op = **um único time**. Em vez de inventar uma estrutura `coop` nova, reusa-se `StructureTeams` com **`TeamCount = 1`**: o ramo `StructureTeams` do `composeTeams` já produz, para `TeamCount=1`, **um** `TeamAssignment` (`team-1`) com todos os `teamSize` jogadores. Assim `composeTeams` **não muda** e a mudança fica inteira no `Format`. O rótulo `team-1` é cosmético (há um só time); o que importa é que todos compartilham um `team_id`, carregado no connect token e carimbado no resultado coletivo (C2).

## Comportamento

### Derivar a formação do time do perfil `coop_mission`

- WHEN um jogador entra na fila (`FindMatch`) de um jogo cujo perfil é `coop_mission` → THE SYSTEM SHALL derivar `MatchFormat{ Structure: "teams", TeamCount: 1, TeamSize: teamSize, Size: teamSize }`.
- IF `teamSize < 2` (perfil corrompido; a validação do C0 exige `≥ 2`) → **ErrProfileUnavailable** (o ticket não entra na fila — mesmo tratamento dos demais arquétipos a tamanho inválido).
- THE SYSTEM SHALL continuar derivando `queue_arena` (`ffa`/`teams`) e `battle_royale` (`ffa`/`squads`) **sem regressão**; `host_world_dropin` segue **ErrProfileUnavailable** no `Format` (não usa fila — usa invasão/sinais).

### Formar o time e provisionar

- WHEN a fila de um jogo `coop_mission` acumula `Size` (= `teamSize`) tickets → THE SYSTEM SHALL formar o grupo (agrupamento FIFO por jogo, **inalterado** — reusa `tryForm`) e provisionar a sessão com **um** `TeamAssignment` (`team-1`) contendo os `teamSize` jogadores (**reusa** `composeTeams`/`StructureTeams`).
- THE SYSTEM SHALL manter a composição de `queue_arena` e `battle_royale` **inalterada**.

## Contratos de Interface

### gRPC

Sem mudança no `.proto`: `FindMatch(game_id)` é a porta de entrada. O `game_id` determina o perfil e, com ele, a formação.

### Domínio

**Inalterado.** `MatchFormat` já tem `Structure`/`TeamCount`/`TeamSize`/`Size`. Nenhuma constante de estrutura nova (reusa `StructureTeams`).

### Adapter `registryclient.Format` — extensão

- Novo `case archetypeCoopMission` (`archetypeCoopMission = "coop_mission"`) na derivação; lê `p.Params.TeamSize` (já presente no `profileResponse`), valida `≥ 2`, monta `MatchFormat{teams, TeamCount:1, TeamSize, Size:teamSize}`.

### `composeTeams` (matchmaker)

**Inalterado** — o ramo `StructureTeams` com `TeamCount=1` já produz o time único.

## Modelo de Dados

Nenhum — matchmaking é stateless (fila em memória). Sem migration, sem store.

## Casos de Teste Obrigatórios

- [ ] (registryclient) perfil `coop_mission` `teamSize=4` → `MatchFormat{teams, TeamCount=1, TeamSize=4, Size=4}`
- [ ] (registryclient) `coop_mission` com `teamSize<2` (ex.: 1) → `ErrProfileUnavailable`
- [ ] (registryclient) `queue_arena`/`battle_royale` seguem derivando certo (sem regressão); `host_world_dropin` → `ErrProfileUnavailable`
- [ ] (composeTeams) formação `teams` `TeamCount=1, TeamSize=4` → 1 assignment `team-1` com os 4 jogadores (garante que o caso degenerado funciona)
- [ ] (matchmaker, integração leve) fila de um jogo `coop_mission` enche `teamSize` tickets → forma 1 grupo, provisiona com o time único (mock do provisioner) — reusa o harness do matchmaker

## Fora do escopo

- **Resultado coletivo** (`collective`) — fatia C2 (session).
- **Bot-fill / lobby-timeout** (formar com menos que `teamSize` humanos, resto vira bot no data plane) — Abertura do roadmap (MVP espera o time cheio, como o `queue_arena`/`battle_royale`).
- **Join-in-progress, lobby/browser filtrado por missão+dificuldade** — Aberturas do roadmap (partilham gap do `host_world_dropin`).
- **Skill/SBMM, região, party** — follow-up (mesmo dos demais arquétipos).
- **session-service** — não é tocado (formação agnóstica de arquétipo).
