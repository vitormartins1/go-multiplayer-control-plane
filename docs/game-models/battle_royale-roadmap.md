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

O `unity-battle-royale-data-plane` (Unity) **não está sequer planejado** em [REPO-TOPOLOGY.md](../REPO-TOPOLOGY.md) — não há server que encolha a zona nem fixe colocações. Consequência (mesmo princípio dos outros arquétipos: **control plane primeiro**):

- Todo BR0–BR3 se **valida no control plane** com o adapter `static` do `ServerAllocator` (server mockado) e um **cliente de teste** simulando o server do BR (enche o lobby, chama `ReportResult` com as colocações como o Unity chamaria).
- A validação **end-to-end real** (Agones + Unity com zona/eliminação de verdade, lobby grande) é etapa **cross-repo posterior**. Este roadmap entrega o **contrato** (`battle_royale-dataplane-contract.md`, fatia do fim) que ela vai consumir.

---

## BR0 — Declaração do perfil (a base)

**Objetivo:** o jogo declara `battle_royale` com seus parâmetros; o console cadastra/edita. Espelha a declaração que abriu o `host_world_dropin` (commit `6fbb3cf`).

**Specs:** `game-registry/007-battle-royale-profile` + `admin-bff`/`admin-ui` (numeração seguinte livre).

**Escopo:**
- **game-registry:** novo `BattleRoyaleParams{ LobbySize, TeamSize, RespawnMode, PlacementPoints bool, ScoringStats[] }` (schema fino na spec), ponteiro `BattleRoyale` no `MultiplayerProfile`, `ArchetypeBattleRoyale`, validação por arquétipo (`profile_service.go`), persistência JSONB (a coluna já existe, migration `005`), leitura no `GetMultiplayerProfile`.
- **admin-bff:** passthrough cru (o BFF já é passthrough do perfil, spec `admin-bff/012`).
- **admin-ui:** ramo condicional no form de perfil (aba Configurações) — select de `battle_royale`, campos `lobbySize`/`teamSize`/`respawnMode`/scoring com tooltips, como os Fieldsets do `host_world_dropin`.

**Depende de:** nada. **Tamanho:** **M** (aditivo puro, padrão já trilhado 2×). **Destrava:** todas as demais (o perfil é lido por matchmaking/session).

---

## BR1 — Lobby grande + composição de squads (matchmaking)

**Objetivo:** o matchmaking enche um **lobby de N grande** (do perfil) e compõe **squads** (solo/duo/quad), completando squads incompletos com solos. Extensão em escala do `queue_arena` NvN, não um pipeline novo.

**Specs:** `matchmaking/008-battle-royale-lobby`.

**Escopo (matchmaking-service):**
- Derivar `lobbySize`/`teamSize` do perfil `battle_royale` (reusa o leitor de perfil que o `matchmaking/004` já usa para derivar o tamanho do match).
- Encher o lobby até `lobbySize` (alvo alto) antes de abrir a sessão; compor squads de `teamSize` (solo = `teamSize` 1). `FindMatch` continua a porta de entrada; ticket ganha o modo (solo/duo/squad) quando `teamSize > 1`. Party real (grupos pré-formados) = follow-up.
- Chama `session.Create(matchID, gameID, teams)` com os squads como `TeamAssignment` — **reusa** a composição do `session/010` (a sessão já é dona canônica do `teamId`). FFA (solo) = N times de 1.

**Depende de:** BR0 (perfil). **Tamanho:** **M–L** (reusa `matchmaking/004`+`005`). **Nota:** skill/SBMM e região ficam **follow-up** (como no `queue_arena`).

---

## BR2 — Resultado por colocação ordinal (session + fan-out) — o núcleo

**Objetivo:** o server do BR reporta as **colocações de todos** no fim; o outcome vira o kind `placement`; o fan-out o distribui. Fecha o "gap central" (colocação ordinal) para este arquétipo.

**Specs:** `session/015-placement-result`.

**Escopo (session-service):**
- Novo kind de outcome: `MatchOutcome.Kind = "placement"`, ponteiro `Placement *PlacementResult`. `PlacementResult{ Entries []PlacementEntry }`, `PlacementEntry{ PlayerID, TeamID, Placement int, SurvivedMs int64, Stats map[string]int64 }` (kills/etc. em `Stats`, reusa o padrão do `PlayerOutcome`). Validação: colocações consistentes (1..N, squad compartilha colocação, sem buracos — regra pura testável).
- `ReportResult` aceita o novo kind (terminal, como o `queue_arena` — decisão #1); publica no `match-results` com o envelope `kind` já existente (o payload muda, o mecanismo não). `ErrInvalidOutcome` em colocação inconsistente.
- **Sem** nova operação de sessão (sem `ReportElimination` — decisão #1: eliminação progressiva é data plane; tracking ao vivo é follow-up).

**Depende de:** BR1 (sessão com os squads). **Tamanho:** **M** (aditivo ao `MatchOutcome`; o mecanismo de fan-out não muda). **Destrava:** BR3.

---

## BR3 — Colher: conquistas + leaderboard + analytics (sinks)

**Objetivo:** os sinks reagem à colocação. Reusa os consumers já discriminados por `kind` (ADR-0020) e os boards cumulativos/por-métrica.

**Specs:** `achievements/010` + `leaderboard/005` + `analytics/005`.

**Escopo:**
- **achievements (consumer):** despacha `placement` → event types por faixa de colocação: `battle_royale_win` (colocação 1), `battle_royale_top_n` (colocação ≤ limiar declarado). **Reusa** o critério por-stat/limiar da A1 (`achievements/008`) para kills/sobrevivência. Idempotência por `matchID` (resultado terminal — reusa `processed_matches`).
- **leaderboard (consumer):** boards do `battle_royale` via `ZINCRBY` cumulativo (reusa A2, `leaderboard/004`): `wins` (1º lugar), `top_n_finishes`, `kills_total`; e/ou highscore de `survivedMs`. Idempotente por `matchID`.
- **analytics (consumer):** despacha `placement` → distribuição de colocações por jogo (histograma 1º/top-10/resto) e média de sobrevivência. JSON aditivo, idempotente por `matchID`.

**Depende de:** BR2 (outcome `placement`). **Tamanho:** **L** (3 sinks, mas cada um reusa padrão existente).

---

## BR4 — Surfacing no console (+ contrato do data plane)

**Objetivo:** o painel mostra o BR — o lobby ao vivo e as estatísticas de colocação; e o contrato control↔data plane fica escrito.

**Specs:** `admin-bff` + `admin-ui` (numeração seguinte livre) + doc `battle_royale-dataplane-contract.md`.

**Escopo:**
- **Rooms:** a sala de um jogo `battle_royale` mostra o lobby (N/`lobbySize`) e o modo (solo/squad). Reusa o board de Rooms (`session/007`). *Vivos/eliminados ao vivo = follow-up (decisão #1).*
- **Analytics:** bloco condicional por perfil (reusa o surfacing da B2/H3) — distribuição de colocações, taxa de "top 10", sobrevivência média.
- **Contrato:** `battle_royale-dataplane-contract.md` (par do `queue_arena`/`host_world_dropin`) — o que o server do BR chama: recebe o lobby via connect token (com `team_id` do squad), roda a partida com o `respawnMode` declarado, chama `ReportResult` com o `PlacementResult`. Fonte de verdade das mudanças cross-repo no futuro `unity-battle-royale-data-plane`.

**Depende de:** BR1 (Rooms lê lobby), BR3 (analytics lê colocações). **Tamanho:** **M**.

---

## Resumo de sequência

| Fatia | Specs | Tamanho | Depende |
|---|---|---|---|
| BR0 — declaração do perfil | game-registry/007 + admin-bff/admin-ui | M | — |
| BR1 — lobby grande + squads | matchmaking/008 | M–L | BR0 |
| BR2 — resultado por colocação | session/015 | M | BR1 |
| BR3 — colher (3 sinks) | achievements/010 + leaderboard/005 + analytics/005 | L | BR2 |
| BR4 — console + contrato | admin-bff + admin-ui + doc | M | BR1, BR3 |

Concluídas BR0–BR4, o `battle_royale` está **completo no control plane** (declarar → encher lobby/squads → reportar colocações → colher → console), validado com server mockado. Próxima etapa (cross-repo): o `unity-battle-royale-data-plane` (Unity) encarnando a zona/eliminação, consumindo o contrato de BR4.

## Aberturas que este roadmap **não** cobre (declaradas, adiadas)

- **Tracking de eliminação ao vivo** (Rooms mostrando vivos/eliminados em tempo real): exige a sessão receber eventos de eliminação incrementais — reabre a decisão #1. Fica como fatia futura se o surfacing ao vivo justificar.
- **Skill/SBMM e região** no lobby grande: mesma abertura do `queue_arena`; latência importa mais aqui (N grande), mas segue follow-up.
- **Party real** (grupos pré-formados entrando juntos no mesmo squad): BR1 compõe squads no matchmaking, mas não honra parties externas — follow-up.
- **Respawn `arena_1v1` (Gulag)/`reboot_timer` como fluxo do control plane:** por ora é 100% data plane (decisão #2). Se algum modo exigir o control plane mediar (ex.: realocar server para o "gulag"), vira ADR próprio.
- **Modo de respawn afetando a colocação provisória no backend:** hoje o backend só recebe a colocação **final**. Idem — só muda se houver requisito de tracking ao vivo.
