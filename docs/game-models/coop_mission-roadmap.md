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

O `unity-coop-mission-data-plane` (Unity) **não está planejado** em [REPO-TOPOLOGY.md](../REPO-TOPOLOGY.md) — não há server com AI Director que cumpra objetivo ou detecte wipe. Consequência (mesmo princípio dos outros arquétipos: **control plane primeiro**):

- Todo C0–C3 se **valida no control plane** com o adapter `static` do `ServerAllocator` (server mockado) e um **cliente de teste** simulando o server da missão (forma o time, chama `ReportResult` com o outcome coletivo como o Unity chamaria).
- A validação **end-to-end real** (Agones + Unity com objetivo/wipe/bots) é etapa **cross-repo posterior**. Este roadmap entrega o **contrato** (`coop_mission-dataplane-contract.md`, fatia C4) que ela vai consumir.

---

## C0 — Declaração do perfil (a base) ✅ main

**Objetivo:** o jogo declara `coop_mission` com seus parâmetros; o console cadastra/edita. Espelha a declaração que abriu `host_world_dropin` (commit `6fbb3cf`) e `battle_royale` (BR0, `game-registry/007`).

> **Status:** implementado na branch `feat/game-registry/008-coop-mission-profile` (specs `game-registry/008` + admin-ui). `CoopMissionParams{TeamSize, Missions, Difficulties, BotfillEnabled, ScoringStats}`, `ArchetypeCoopMission`, validação (`teamSize ≥ 2`, catálogos como identificadores estáveis sem duplicata), JSONB sem migration. Form `coop_mission` no `MultiplayerSettings.tsx` + union em `api.ts`. Verde: unit 119 (`-race`, service 89.4%), integração testcontainers 20, lint 0, admin-ui build. Diferente da BR (form saiu só na BR4), aqui a UI já entrou junto.

**Specs:** `game-registry/008-coop-mission-profile` + `admin-ui` (numeração seguinte livre).

**Escopo:**
- **game-registry:** novo `CoopMissionParams{ TeamSize int, Missions []string, Difficulties []string, BotfillEnabled bool, ScoringStats []string }` (schema fino na spec), ponteiro `CoopMission *CoopMissionParams` no `MultiplayerProfile`, constante `ArchetypeCoopMission = "coop_mission"`, validação por arquétipo em `profile_service.go` (`teamSize >= 2`; `Missions`/`Difficulties` opcionais mas sem vazios/duplicados se presentes; `ScoringStats` sem vazios/duplicados). Persistência JSONB — **a coluna já existe** (migration `005`), sem migration nova. Encode/decode no repo e ramo no transport: os **quatro switches** do padrão tagged-union (service-validate / repo-encode / repo-decode / transport).
- **admin-ui:** ramo condicional no form de perfil (`MultiplayerSettings.tsx`, aba Configurações) — opção `coop_mission` no select, campos `teamSize`/`missions`/`difficulties`/`botfill`/`scoringStats` com tooltips, espelhando os Fieldsets do `battle_royale`; **union em `api.ts`** ganha a variante `coop_mission`. Verificação: `cd services/admin-ui && npm run build`.
- **admin-bff:** **nada** — o passthrough de perfil é `json.RawMessage` cru; o `coop_mission` já flui sem tipar (achado da BR4).

**Depende de:** nada. **Tamanho:** **M** (aditivo puro, padrão já trilhado 3×). **Destrava:** todas as demais (o perfil é lido por matchmaking/session).

---

## C1 — Formação do time co-op (matchmaking) ✅ main

**Objetivo:** o matchmaking forma **um único time** de `teamSize` (do perfil) e abre a sessão. Caso degenerado da composição NvN do `queue_arena` — um lado só.

> **Status:** implementado na branch `feat/matchmaking/009-coop-mission-team` (spec `matchmaking/009`). Reuso máximo: co-op = `StructureTeams` com `TeamCount=1` → `composeTeams`/domínio/**session intactos** (confirmou a descoberta da BR1). A mudança é um único `case archetypeCoopMission` no `registryclient.Format` (valida `teamSize ≥ 2`, monta `{teams, TeamCount:1, TeamSize, Size:teamSize}`). Verde: unit 113 (`-race`), service 93.4%/registryclient 92.6%, lint 0.

**Specs:** `matchmaking/009-coop-mission-team`.

**Escopo (matchmaking-service):**
- Derivar `teamSize` do perfil `coop_mission` (reusa o leitor de perfil do `matchmaking/004`/`registryclient` + cache).
- Encher a fila até `teamSize` e compor **um `TeamAssignment` único** (todos no mesmo `teamId`, ex. `team-coop`). `FindMatch` continua a porta de entrada. Chama `session.Create(matchID, gameID, teams)` com o time único — **reusa** a composição do `session/010` (dono canônico do `teamId`). **session provavelmente intacto** (a formação já é agnóstica de arquétipo — a diferença do `coop_mission` é o *resultado*, C2 — mesmo achado da BR1).
- Bot-fill **não** entra aqui: se faltam humanos, é o data plane que preenche com bots (decisão #2). O match forma com os humanos disponíveis (mínimo a definir no plan — provável: forma com ≥1 e o resto vira bot no server).

**Depende de:** C0 (perfil). **Tamanho:** **S–M** (reusa `matchmaking/004`+`005`; menor que BR1, sem squads ragged). **Nota:** lobby/browser, join-in-progress, skill/região = **Aberturas** (follow-up).

---

## C2 — Resultado coletivo (session + fan-out) — o núcleo ✅ main

**Objetivo:** o server reporta **um desfecho coletivo** (`MISSION_SUCCESS`/`MISSION_FAILURE`) + stats por jogador no fim; o outcome vira o kind `collective`; o fan-out o distribui. Fecha o **último eixo** do "gap central" (resultado coletivo) e completa os quatro modelos do ADR-0023.

> **Status:** implementado na branch `feat/session/016-collective-result` (spec `session/016`). Kind `collective` no `MatchOutcome` (4º ponteiro `Collective`), proto regenerado com `buf` (`CollectiveResult{outcome, players[]}`/`CollectivePlayer{playerId, teamId, stats}` no `oneof`). O **ponto novo** confirmado no código: `Outcome` vive no nível da sessão (campo do `CollectiveResult`), não por jogador. Dispatch por kind em `validateOutcome`/`stampTeams`/`toMatchOutcome`/`SNS.Publish`/`Log.Publish`; envelope `kind=collective` no mesmo tópico `match-results`. Validação estrutural: `players` não-vazio + `outcome ∈ {MISSION_SUCCESS, MISSION_FAILURE}`; valores de `stats` **não** checados (server autoritativo, como o `placement`). Terminal, reusa `stampTeams`→publish→closeSession. Verde: 126 testes `-race`, service 94.4%, lint 0. **Nada novo de Go** (tagged-union + dispatch por kind já documentados). Sem ADR novo.

**Specs:** `session/016-collective-result`.

**Escopo (session-service):**
- Novo kind de outcome: `MatchOutcome.Kind = "collective"` (constante `OutcomeCollective`), ponteiro `Collective *CollectiveResult`. Tipos:
  ```
  CollectiveResult{ Outcome string; Players []CollectivePlayer }   // Outcome no nível da SESSÃO
  CollectivePlayer{ PlayerID string; TeamID string; Stats map[string]int64 }
  ```
  Constantes `MissionSuccess = "MISSION_SUCCESS"`, `MissionFailure = "MISSION_FAILURE"`.
- **Proto:** adicionar `CollectiveResult`/`CollectivePlayer` ao `oneof outcome` e **regenerar com `buf generate`** — nunca editar `.pb.go` à mão.
- **Dispatch por `kind`** (já existente): mais um ramo em `validateOutcome` (outcome ∈ {SUCCESS, FAILURE}; ≥1 player; stats não-negativos), `stampTeams` (carimba `TeamID` a partir da formação, não confia no eco do server — igual aos outros kinds), `toMatchOutcome`, `SNSPublisher.Publish` e `LogPublisher.Publish`. Envelope `kind=collective` no **mesmo** tópico `match-results` (o payload muda — `outcome` de sessão + `players[].stats` —, o mecanismo não). `ErrInvalidOutcome` em desfecho fora do vocabulário.
- **Sem** nova operação de sessão. Terminal, um `ReportResult`, reusa `stampTeams`→publish→closeSession (`CloseResult`).

**Depende de:** C1 (sessão com o time). **Tamanho:** **M** (aditivo ao `MatchOutcome`; o mecanismo de fan-out não muda). **Destrava:** C3.

---

## C3 — Colher: conquistas + leaderboard + analytics (sinks) ✅ main

**Objetivo:** os sinks reagem ao desfecho coletivo. Reusa os consumers já discriminados por `kind` (ADR-0020) — o padrão exato da BR3.

> **Status:** implementado e mergeado (3 branches, especs `achievements/011` + `leaderboard/006` + `analytics/006`). achievements: `CollectiveEventTypes` → `coop_mission_played`/`_success`/`_failed` (desfecho da sessão, igual para todos), reusa `ApplyMatchResult`/`processed_matches`. leaderboard: board cumulativo `missions_completed` (+1 p/ todos em SUCCESS) + `<stat>_total`/highscore por stat. analytics: migration `005_mission_rollup` (`game_mission_rollup`), `ApplyCollective` (successes/failures idempotente por matchId), `successRate` no read; **corrigiu o gap** de mounts do `analytics-postgres` no compose (002–005). Verde nos três: build, testes `-race` (analytics integração testcontainers), lint 0. Nada novo de Go (dispatch por kind + idempotência por matchId já documentados).

**Specs:** `achievements/011` + `leaderboard/006` + `analytics/006`.

**Escopo:**
- **achievements (consumer):** despacha `collective` → event types: `coop_mission_played` (todos), `coop_mission_success` (todos, se `MISSION_SUCCESS`) / `coop_mission_failed` (todos, se `MISSION_FAILURE`). **Reusa** o critério por-stat da A1 (`achievements/008`) para kills/revives/minério. Idempotência por `matchID` (terminal — reusa `processed_matches`, sem migration).
- **leaderboard (consumer):** boards do `coop_mission` via `ZINCRBY` cumulativo (reusa A2, `leaderboard/004`): `missions_completed` (só em SUCCESS), `<stat>_total` por stat declarada. Idempotente por `matchID` (script Lua). Sem migration (Redis).
- **analytics (consumer):** despacha `collective` → rollup `game_mission_rollup` (contagem de sucesso/falha por jogo + taxa de sucesso). **Migration nova** (próximo número na sequência do analytics; a BR3 usou a 004) — montar no compose (gotcha conhecido). Idempotente por `matchID` (`processed_matches`), JSON aditivo no `GameAnalytics` (render = C4).

**Depende de:** C2 (outcome `collective`). **Tamanho:** **L** (3 sinks, cada um reusa padrão existente).

---

## C4 — Surfacing no console (+ contrato do data plane) ✅ branch

**Objetivo:** o painel mostra o `coop_mission` — a taxa de sucesso/falha das missões e o modo co-op no Rooms; e o contrato control↔data plane fica escrito.

> **Status:** implementado na branch `feat/coop-mission/c4-console` (specs `admin-bff/016` + `admin-ui/022` + doc). admin-bff propaga o campo `mission` no `GameAnalyticsDetail` (passthrough; perfil já fluía cru). admin-ui: bloco "Desfecho das missões" (cumpridas/falhas/taxa) em `GameAnalytics.tsx` + badge `co-op N/teamSize` em `Rooms.tsx` (cache de perfil por gameId) + tipo `mission` em `api.ts`. O form de perfil já saíra na C0. Contrato: `docs/game-models/coop_mission-dataplane-contract.md` (par dos 3 existentes). Verde: admin-bff testes + lint 0; admin-ui `npm run build`.

**Specs:** `admin-bff` + `admin-ui` (numeração seguinte livre) + doc `coop_mission-dataplane-contract.md`.

**Escopo:**
- **admin-bff:** propaga o campo de missão no `GameAnalyticsDetail` (sucesso/falha/taxa) — **achado da BR4:** só o analytics precisa de campo; perfil já flui cru. Passthrough.
- **admin-ui:** bloco condicional na Analytics (`GameAnalytics.tsx`) com sucesso/falha/taxa de missão; rótulo `co-op · teamSize` no Rooms (`Rooms.tsx`, cache de perfil por `gameId`, como o BR). *(O form de perfil já saiu em C0 — aqui é só surfacing.)*
- **Contrato:** `coop_mission-dataplane-contract.md` (par do `queue_arena`/`host_world_dropin`/`battle_royale`): o server recebe o time via connect token (com `team_id` do time co-op), roda a missão com os params declarados (`missions`/`difficulties`/`botfill`), chama `ReportResult` com o `CollectiveResult` terminal (`MISSION_SUCCESS/FAILURE` + stats). Envelope `kind=collective`. Fonte de verdade das mudanças cross-repo no futuro `unity-coop-mission-data-plane`.

**Depende de:** C1 (Rooms lê o perfil), C3 (analytics lê o desfecho). **Tamanho:** **M**.

---

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
