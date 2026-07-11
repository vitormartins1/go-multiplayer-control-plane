# Roadmap — arquétipo `coop_mission` ponta a ponta (ADR-0023)

> **Propósito:** planejar em lote as fatias que levam o arquétipo `coop_mission` de **inexistente** (nem os params estão no game-registry) a **comportamento ponta a ponta no control plane** — o jogo declara um co-op de time pequeno contra o ambiente, o matchmaking forma **um único time**, a sessão roda num server, o server reporta **um desfecho coletivo** (`MISSION_SUCCESS`/`MISSION_FAILURE` igual para todos) com stats individuais, os sinks colhem e o console mostra. Espelha o [`battle_royale-roadmap.md`](./battle_royale-roadmap.md) e o [`host_world_dropin-roadmap.md`](./host_world_dropin-roadmap.md): fonte única de estado é a tabela "Estado da implementação" do [README](./README.md); aqui só o **que falta**.
>
> **Origem do modelo:** ficha [04 — Co-op PvE de missão](./04-coop-pve-mission.md). Leia-a antes de abrir a primeira spec (em especial §5 e §6). O arquétipo traz **o último eixo de resultado que faltava**: **resultado coletivo** — o desfecho é **da sessão inteira** (todos ganham ou perdem juntos), diferente de `win_loss_draw` (por jogador — `queue_arena`), `placement` (colocação ordinal — `battle_royale`) e `participation_event` (por evento/papel — `host_world_dropin`), **os três já completos no control plane**. Com ele, os quatro modelos de resultado do ADR-0023 ficam cobertos.
>
> Ordem recomendada: **C0 → C1 → C2 → C3 → C4 (+ contrato)**. O núcleo é **C2** (o outcome coletivo); as demais reusam a infra polimórfica já existente.

## Ponto de partida (o que já está na `main`)

- ⬜ **Nada.** Como foi o `battle_royale` na largada, o `coop_mission` **nem tem declaração**: `MultiplayerProfile` conhece só `queue_arena`, `host_world_dropin` e `battle_royale` (`game-registry/internal/domain/game.go`). O enum de arquétipos do ADR-0023 nomeia `coop_mission` e o kind de resultado `collective`, mas **nenhum serviço o modela** (não há `ArchetypeCoopMission`, nem `MatchOutcome.Collective`).
- ✅ **Reaproveitável (o essencial):** a infra polimórfica foi projetada exatamente para isto e já absorveu **três** kinds — `MatchOutcome{Kind, *ptr}` (session, tagged union por ponteiro: `WinLossDraw` + `Placement`), envelope de fan-out discriminado por `kind` (ADR-0020), **dispatch por `kind` já presente** em `validateOutcome`/`stampTeams`/`toMatchOutcome`/`SNS.Publish`/`Log.Publish` (introduzido na BR2) e **nos três sinks** (achievements/leaderboard/analytics, BR3). O `coop_mission` **adiciona um quarto ponteiro** (`Collective`) e mais um ramo em cada switch — não reescreve o modelo. Composição de times do `matchmaking/005` + `session/010` (dono canônico do `teamId`), form de perfil condicional do `admin-ui` e passthrough cru do perfil no `admin-bff` (`json.RawMessage`) também são reusados.

## Fronteira importante (o que o control plane **não** modela)

Da ficha §6 — deixar explícito evita inchar o backend: **IA/AI Director, hordas, incapacitação/revive, geração procedural, os bots em si = data plane (gameplay).** O control plane só conhece: a formação do time (matchmaking), a sessão (1 server/partida), os **params de missão declarados** (que o data plane lê) e o **resultado coletivo** (terminal). O server decide *quando* a missão foi cumprida ou houve wipe; o control plane só recebe o desfecho coletivo final + stats.

## Escopo deliberado: este roadmap entrega o **eixo de resultado**, não a sessão host-cêntrica

A ficha 04 lista vários gaps além do resultado coletivo: **matchmaking por lobby/browser filtrado**, **join-in-progress**, **bot-fill**, **missão+dificuldade carregadas na sessão**. **A maioria já foi enfrentada** (ou é follow-up conhecido) no `host_world_dropin`: injeção/entrada numa sessão existente (H1a/H1b ✅), sessão host-cêntrica com slots (H0 ✅), senha/pools (follow-up). **Este roadmap NÃO os reabre.** O `coop_mission` aqui prova **o modelo de resultado coletivo** — o gap que só ele fecha — sobre a formação **queue-form** que já temos. Ver "Aberturas declaradas, adiadas" no fim: join-in-progress/bot-fill/lobby-browser ficam explicitamente fora, para não fundir dois gaps grandes numa fatia só.

## Tensões de modelagem a decidir no `plan.md` (não bloquear agora)

Três decisões estruturais. **Recomendação anotada; confirmar por escrito no plan da fatia dona.**

1. **O desfecho vive no nível da SESSÃO, não do jogador (dona de C2 — o ponto novo).** Nos três kinds atuais o desfecho é *por participante*: `PlayerOutcome.Outcome` (win/loss/draw), `PlacementEntry.Placement`, `ParticipationEvent` por jogador. No `coop_mission` **há um único desfecho para todos**. → **Recomendação:** modelar `CollectiveResult{ Outcome string /* MISSION_SUCCESS|MISSION_FAILURE */, Players []CollectivePlayer }`, onde `Outcome` é **um campo do resultado** (não de cada jogador) e `CollectivePlayer{ PlayerID, TeamID, Stats }` carrega **só stats** (kills/revives/minério) que **modulam recompensa, não o desfecho**. É a diferença estrutural que justifica o arquétipo: o `Outcome` "sobe" de `PlayerOutcome` para o struct do resultado. Terminal (um `ReportResult` no fim), como `queue_arena`/`battle_royale` — **não** é evento incremental (isso é `participation_event`/`host_world_dropin`).

2. **Missão + dificuldade + bot-fill são params declarados, não lógica de control plane (dona de C0).** Que missões/dificuldades o host pode escolher, e se bots preenchem vagas, são conteúdo que o **data plane** consome — igual ao `respawnMode` do `battle_royale`. → **Recomendação:** `CoopMissionParams` **armazena e repassa** (`Missions[]`, `Difficulties[]`, `BotfillEnabled`); o control plane **não implementa** seleção de missão nem spawn de bot. Mantém C0 barato (mais um ramo no tagged-union do perfil, JSONB sem migration).

3. **Co-op = um único time; formação por queue-form (dona de C1).** O `matchmaking/005` já forma lados NvN e o `session/010` é dono canônico do `teamId`. Co-op é o caso degenerado: **um único time** de `teamSize`. → **Recomendação:** C1 deriva `teamSize` do perfil e forma **um `TeamAssignment` único** (todos no mesmo `teamId`), reusando o pipeline; **não** inventa lobby/browser nem join-in-progress (Aberturas). FFA não se aplica (é co-op puro, um lado só).

## Realidade cross-repo: o data plane não existe ainda

O `unity-coop-mission-data-plane` (Unity) **ainda não foi criado** — não há server com AI Director que cumpra objetivo ou detecte wipe. Todo o roadmap abaixo foi validado **só no control plane**, com o adapter `static` do `ServerAllocator` (server mockado) e um cliente de teste simulando o server da missão. A validação end-to-end real (Agones + Unity com objetivo/wipe/bots) é etapa cross-repo futura — o contrato que ela vai consumir já está escrito ([`coop_mission-dataplane-contract.md`](./coop_mission-dataplane-contract.md)).

## Fatias executadas (histórico resumido — detalhe completo nas specs)

Todas as 5 fatias (C0–C4) foram concluídas e validadas com server mockado — cada uma reaproveitou um pipeline já trilhado pelo `queue_arena`/`battle_royale`, sem inventar mecanismo novo:

- **C0 — declaração do perfil** (`game-registry/008` + `admin-ui`): `CoopMissionParams` (teamSize/missions/difficulties/botfillEnabled/scoringStats) no perfil de multiplayer, form condicional no console desde já (diferente do BR, cujo form só saiu na BR4).
- **C1 — formação do time co-op** (`matchmaking/009`): caso degenerado da composição NvN do `queue_arena` — um único time (`TeamCount=1`), `session-service` intacto.
- **C2 — resultado coletivo** (`session/016`) — **o núcleo desta fatia**: novo kind `collective` no `MatchOutcome` polimórfico — o desfecho (`MISSION_SUCCESS`/`MISSION_FAILURE`) vive no nível da **sessão**, não do jogador (diferença estrutural dos outros 3 kinds). Com ele, os 4 modelos de resultado do ADR-0023 (`win_loss_draw`, `placement`, `participation_event`, `collective`) ficam cobertos.
- **C3 — sinks** (`achievements/011` + `leaderboard/006` + `analytics/006`): os 3 consumers do fan-out reagem ao desfecho coletivo — conquista por sucesso/falha (igual para todos), board cumulativo `missions_completed`, rollup de taxa de sucesso.
- **C4 — console + contrato** (`admin-bff/016` + `admin-ui/022` + doc): Analytics/Rooms mostram o `coop_mission`; contrato cross-repo escrito.

## Resumo de sequência

| Fatia | Specs | Tamanho | Depende |
|---|---|---|---|
| C0 — declaração do perfil ✅ main | game-registry/008 + admin-ui | M | — |
| C1 — formação do time co-op ✅ main | matchmaking/009 | S (session intacto — confirmado) | C0 |
| C2 — resultado coletivo **(núcleo)** ✅ main | session/016 | M | C1 |
| C3 — colher (3 sinks) ✅ main | achievements/011 + leaderboard/006 + analytics/006 | L | C2 |
| C4 — console + contrato ✅ main | admin-bff/016 + admin-ui/022 + doc | M | C1, C3 |

✅ **C0–C4 concluídas (todas na main)** — o `coop_mission` está **completo no control plane** (declarar → formar time → reportar desfecho coletivo → colher → console), validado com server mockado. Com o kind `collective`, os **quatro modelos de resultado** do ADR-0023 (`win_loss_draw`, `placement`, `participation_event`, `collective`) ficam cobertos. Próxima etapa (cross-repo): o `unity-coop-mission-data-plane` (Unity) com AI Director/objetivo/wipe, consumindo o [contrato de C4](./coop_mission-dataplane-contract.md).

## Aberturas que este roadmap **não** cobre (declaradas, adiadas)

- **Join-in-progress + lobby/browser filtrado** (entrar numa missão em andamento, descoberta por missão+dificuldade): partilha o gap do `host_world_dropin` (injeção já feita em H1a/H1b; browser do queue-form segue aberto). Fica como fatia futura — não fundir com o eixo de resultado.
- **Bot-fill como modelo do control plane** (slots de humano vs. bot): por ora 100% data plane (decisão #2). Só entra se algum requisito exigir o control plane mediar (ex.: reservar slot para humano tardio).
- **Missão + dificuldade carregadas *na sessão*** (não só declaradas no perfil): hoje são params declarados que o data plane lê; carregá-las como estado da sessão (para o Rooms mostrar "missão X, Hazard 4") é surfacing futuro.
- **Stats individuais modulando recompensa de fato:** o `collective` carrega os stats; a *regra* de recompensa (XP/loot por performance) é progressão de gameplay, fora do control plane.
- **Topologia `p2p`** (L4D/DRG são host-hospedados): campo do perfil (ADR-0023), implementação P2P = ADR futuro, como nos demais arquétipos.
