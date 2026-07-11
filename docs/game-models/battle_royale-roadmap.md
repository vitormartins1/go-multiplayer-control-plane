# Roadmap — arquétipo `battle_royale` ponta a ponta (ADR-0023)

> **Propósito:** planejar em lote as fatias que levam o arquétipo `battle_royale` de **inexistente** (nem os params estão no game-registry) a **comportamento ponta a ponta no control plane** — o jogo declara um lobby grande com modo de respawn, o matchmaking enche o lobby (solo/squad), a sessão roda com muitos jogadores num server, o server reporta as **colocações** de todos e o console mostra. Espelha o [`host_world_dropin-roadmap.md`](./host_world_dropin-roadmap.md) e o [`queue_arena-roadmap.md`](./queue_arena-roadmap.md): fonte única de estado é a tabela "Estado da implementação" do [README](./README.md); aqui só o **que falta**.
>
> **Origem do modelo:** ficha [03 — Battle Royale](./03-battle-royale.md). Leia-a antes de abrir a primeira spec: o arquétipo traz **dois eixos novos** — **colocação ordinal** (1º..Nº, nem `Won bool` nem WIN/LOSS/DRAW) e **eliminação progressiva** (o jogador "acaba" antes do fim, vira espectador, mas a sessão segue). Também exercita **escala de sessão** (lobby grande num server).
>
> Ordem recomendada: **BR0 → BR1 → BR2 → BR3 → BR4 (+ contrato)**. O arquétipo `coop_mission` fica fora deste roadmap.

## Ponto de partida (o que já está na `main`)

- ⬜ **Nada.** Diferente do `host_world_dropin`, o `battle_royale` **nem tem declaração**: `MultiplayerProfile` só conhece `queue_arena` e `host_world_dropin` (`game-registry/internal/domain/game.go`). O enum de arquétipos do ADR-0023 lista `battle_royale`, mas nenhum serviço o modela.
- ✅ **Reaproveitável:** a infra polimórfica já existe e foi projetada para isto — `MatchOutcome{Kind, *ptr}` (session, tagged union por ponteiro), envelope de fan-out discriminado por `kind` (ADR-0020), composição de times do `queue_arena` (matchmaking `005` + session `010`), boards por métrica/cumulativos do leaderboard (`003`/`004`), critério por-stat das conquistas (`008`). O `battle_royale` **adiciona um ponteiro** (`Placement`), não reescreve o modelo.

## Fronteira importante (o que o control plane **não** modela)

Da ficha §6 — deixar explícito evita inchar o backend: **zona que encolhe, mapa, loot, spawns, o momento físico da eliminação = data plane (gameplay).** O control plane só conhece: lobby (matchmaking), sessão (1 server/partida), o **modo de respawn declarado** (param que o data plane lê) e o **resultado por colocação** (terminal). O server decide *quando* fixa cada colocação segundo o `respawnMode`; o control plane só recebe as colocações finais.

## Tensões de modelagem a decidir no `plan.md` (não bloquear agora)

Três decisões estruturais. **Recomendação anotada; confirmar por escrito no plan da fatia dona.**

1. **Resultado terminal vs. eliminação progressiva ao vivo (dona de BR2).** A ficha lista "eliminação progressiva na sessão" como gap, mas o §3/§5 esclarece: *"sobra 1 → o server reporta as colocações de todos (resultado terminal único, como a Arena — mas ordinal)"*. A colocação é **fixada na eliminação pelo data plane** e enviada **junta no fim**. → **Recomendação:** modelar o `battle_royale` como **resultado terminal por colocação** (um `ReportResult` no fim, igual ao `queue_arena`, com outcome `placement`) — mínima mudança na sessão. O *tracking ao vivo* de "quantos vivos / eliminado às Xs" (para o Rooms mostrar "12/60 vivos") é **surfacing opcional**, fica como fatia/​follow-up posterior, **não** entra no caminho crítico. Mantém a "fronteira importante" acima.

2. **Modo de respawn é comportamento de data plane, não lógica de control plane (dona de BR0).** `one_life`/`teammate_revive`/`arena_1v1`/`reboot_timer` decidem *quando* a colocação é provisória vs. final — decisão do **server** durante a partida. → **Recomendação:** o `respawnMode` é um **param declarado** que o control plane **armazena e repassa** ao data plane (via perfil/contrato); o control plane **não implementa** nenhuma lógica de respawn. Isso mantém BR0 barato (só mais um campo) e honra a fronteira.

3. **Composição de squads = extensão do `teams` do `queue_arena`, em escala (dona de BR1).** O `queue_arena` já forma NvN (`matchSize` derivado do perfil, `TeamAssignment` posicional). O `battle_royale` é o mesmo mecanismo com **alvo N grande** (`lobbySize`) e **muitos times pequenos** (squads de `teamSize`), completando squads incompletos com solos. → **Recomendação:** reusar o pipeline de composição do `matchmaking/005`, parametrizado pelo perfil `battle_royale` (deriva `lobbySize` e `teamSize`); **não** inventar um segundo pipeline. Skill/SBMM e região continuam **fora** (follow-up, como no `queue_arena`).

## Realidade cross-repo: o data plane não existe ainda

O `unity-battle-royale-data-plane` (Unity) **ainda não foi criado** — não há server que encolha a zona nem fixe colocações. Todo o roadmap abaixo foi validado **só no control plane**, com o adapter `static` do `ServerAllocator` (server mockado) e um cliente de teste simulando o server do BR. A validação end-to-end real (Agones + Unity com zona/eliminação de verdade) é etapa cross-repo futura, condicionada a esse repo existir — o contrato que ela vai consumir já está escrito ([`battle_royale-dataplane-contract.md`](./battle_royale-dataplane-contract.md)).

## Fatias executadas (histórico resumido — detalhe completo nas specs)

Todas as 5 fatias (BR0–BR4) foram concluídas e validadas com server mockado — cada uma reaproveitou um pipeline já trilhado pelo `queue_arena`/`host_world_dropin`, sem inventar mecanismo novo:

- **BR0 — declaração do perfil** (`game-registry/007` + `admin-bff`/`admin-ui`): `BattleRoyaleParams` (lobbySize/teamSize/respawnMode/scoringStats) no perfil de multiplayer, form condicional no console.
- **BR1 — lobby grande + squads** (`matchmaking/008`): extensão em escala da composição NvN do `queue_arena` — enche um lobby grande e forma squads, completando com solos.
- **BR2 — colocação ordinal** (`session/015`): novo kind de outcome `placement` (`PlacementResult`, terminal — decisão #1 abaixo), aditivo ao `MatchOutcome` polimórfico já existente.
- **BR3 — sinks** (`achievements/010` + `leaderboard/005` + `analytics/005`): os 3 consumers do fan-out (já discriminados por `kind`, ADR-0020) reagem à colocação — conquista por faixa/limiar, board cumulativo, histograma de colocação.
- **BR4 — console + contrato** (`admin-bff/015` + `admin-ui/021` + doc): Rooms/Analytics mostram o BR; contrato cross-repo escrito.

## Resumo de sequência

| Fatia | Specs | Tamanho | Depende |
|---|---|---|---|
| BR0 — declaração do perfil ✅ main | game-registry/007 + admin-bff/admin-ui | M | — |
| BR1 — lobby grande + squads ✅ main | matchmaking/008 | M (menor: session intacto) | BR0 |
| BR2 — resultado por colocação ✅ main | session/015 | M | BR1 |
| BR3 — colher (3 sinks) ✅ main | achievements/010 + leaderboard/005 + analytics/005 | L | BR2 |
| BR4 — console + contrato ✅ main | admin-bff/015 + admin-ui/021 + doc | M | BR1, BR3 |

✅ **BR0–BR4 concluídas** — o `battle_royale` está **completo no control plane** (declarar → encher lobby/squads → reportar colocações → colher → console), validado com server mockado. Próxima etapa (cross-repo): o `unity-battle-royale-data-plane` (Unity) encarnando a zona/eliminação, consumindo o [contrato de BR4](./battle_royale-dataplane-contract.md).

## Aberturas que este roadmap **não** cobre (declaradas, adiadas)

- **Tracking de eliminação ao vivo** (Rooms mostrando vivos/eliminados em tempo real): exige a sessão receber eventos de eliminação incrementais — reabre a decisão #1. Fica como fatia futura se o surfacing ao vivo justificar.
- **Skill/SBMM e região** no lobby grande: mesma abertura do `queue_arena`; latência importa mais aqui (N grande), mas segue follow-up.
- **Party real** (grupos pré-formados entrando juntos no mesmo squad): BR1 compõe squads no matchmaking, mas não honra parties externas — follow-up.
- **Respawn `arena_1v1` (Gulag)/`reboot_timer` como fluxo do control plane:** por ora é 100% data plane (decisão #2). Se algum modo exigir o control plane mediar (ex.: realocar server para o "gulag"), vira ADR próprio.
- **Modo de respawn afetando a colocação provisória no backend:** hoje o backend só recebe a colocação **final**. Idem — só muda se houver requisito de tracking ao vivo.
