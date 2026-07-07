# Spec: 016-collective-result

**Serviço:** session-service
**Status:** Aprovado (batch C1–C4)
**Data:** 2026-07-07
**ADR:** 0023 (perfil de multiplayer por jogo) — arquétipo `coop_mission`
**Roadmap:** `docs/game-models/coop_mission-roadmap.md` — fatia **C2** (o núcleo)
**Depende de:** C1 (`matchmaking/009` — time co-op) · reusa o `ReportResult` e o fan-out por `kind` (specs `009`/`013`/`015`, ADR-0020)

## Contexto

Terceira fatia do `coop_mission` (ficha `docs/game-models/04-coop-pve-mission.md`) e o **núcleo** do arquétipo: o resultado **coletivo** por sessão. Hoje o `MatchOutcome` tem três kinds — `win_loss_draw` (`session/009`), `placement` (`session/015`/BR2) e o `participation_event` (fan-out do host_world_dropin, `session/013`); todos publicam no tópico `match-results` com um envelope discriminado por `kind`.

Esta fatia adiciona o kind **`collective`**: o server da missão reporta **um desfecho para todo o time** no fim (`MISSION_SUCCESS`/`MISSION_FAILURE`), com stats individuais que **modulam recompensa, não o desfecho**. É **terminal** (um `ReportResult`, como `queue_arena`/`battle_royale`), **não** incremental (isso é `participation_event`). Fecha o **último eixo** do "gap central" (resultado coletivo) e, com ele, os **quatro** modelos de resultado do ADR-0023 ficam cobertos. Os **sinks** que reagem ao `collective` são a fatia **C3** — aqui só produzimos e publicamos.

**O ponto novo (decisão #1 do roadmap):** nos três kinds atuais o desfecho é **por participante**. No `collective` o desfecho vive no **nível da sessão** — é um campo `Outcome` do resultado, não de cada jogador. Essa é a diferença estrutural que justifica o arquétipo.

**Fronteira (ficha §6):** IA/Director, hordas, revive, procedural, bots = data plane. O control plane recebe só o **desfecho coletivo final** + stats.

## Comportamento

### Reportar o resultado coletivo

- WHEN o server chama `ReportResult(matchId, collective{ outcome, players })` com um outcome `collective` **válido** → THE SYSTEM SHALL carimbar o `team_id` canônico de cada jogador (a partir da composição registrada na formação — o time `team-1` da C1), **publicar** no `match-results` com envelope `kind=collective`, e **encerrar** a sessão (`CloseResult`) — exatamente o fluxo terminal do `win_loss_draw`/`placement`.
- IF não houver sessão para o `matchId` → `ErrSessionNotFound` (inalterado).
- THE SYSTEM SHALL manter `win_loss_draw`, `placement` e `participation_event` **sem regressão**.

### Forma do outcome `collective`

Um desfecho de **sessão** + uma entrada por participante:

| Campo | Tipo | Significado |
|---|---|---|
| `outcome` | string | **`MISSION_SUCCESS`** \| **`MISSION_FAILURE`** — desfecho da **sessão inteira**, igual para todos |
| `players[].playerId` | string | jogador |
| `players[].teamId` | string | o time co-op (`team-1`); **carimbado pelo session** a partir da formação, não confiado no que o server ecoa |
| `players[].stats` | map[string]int64 | métricas individuais (kills, revives, ore…) que modulam recompensa; pode ser vazia |

Não há `Won`/`Score`/`placement` por jogador — o desfecho é do time; os stats não decidem o desfecho.

### Validação (estrutural, server é autoritativo)

Como nos demais kinds, a validação é **estrutural** — o game server é autoritativo:

- IF `collective` ausente ou `players` vazio → `ErrInvalidOutcome`.
- IF `outcome ∉ {MISSION_SUCCESS, MISSION_FAILURE}` → `ErrInvalidOutcome`.
- THE SYSTEM SHALL **não** validar os valores de `stats` (server autoritativo — mesma filosofia do `placement`, que não valida os valores de `stats`; só `survivedMs`, um campo dedicado, é checado lá).
- IF nenhum campo do `oneof` estiver setado → `ErrUnsupportedOutcome` (inalterado).

### Carimbo de time (reuso)

- WHEN o outcome é `collective` → THE SYSTEM SHALL sobrescrever o `teamId` de cada jogador com o time registrado na formação (`TeamByPlayer`). É o mesmo `stampTeams` do `win_loss_draw`/`placement`, estendido ao ponteiro `Collective`.

### Fan-out (reuso do envelope por `kind`)

- THE SYSTEM SHALL publicar no **mesmo** tópico `match-results` um envelope `kind=collective` com `schemaVersion`, `matchId`, `gameId`, `outcome` (da sessão) e `players[]` (`playerId`/`teamId`/`stats`). Reusa o mecanismo (SNS real + Log fallback); só o **payload** por kind é novo. Os sinks do `coop_mission` (C3) reagem.

## Contratos de Interface

### gRPC (`session.proto`) — extensão do `oneof`

- `ReportResultRequest.outcome` (oneof) ganha `CollectiveResult collective = 4;`.
- Novas mensagens `CollectiveResult{ string outcome = 1; repeated CollectivePlayer players = 2; }` e `CollectivePlayer{ string player_id = 1; string team_id = 2; map<string,int64> stats = 3; }`.
- **Regenerar com `buf generate`** (de `services/session-service/`; nunca editar o `.pb.go` à mão — gotcha do `go_package`).

### Domínio (session) — extensão

- consts `OutcomeCollective = "collective"`, `MissionSuccess = "MISSION_SUCCESS"`, `MissionFailure = "MISSION_FAILURE"`.
- `MatchOutcome` ganha `Collective *CollectiveResult` (tagged union por ponteiro; exatamente o ponteiro do `Kind` é não-nil).
- `CollectiveResult{ Outcome string; Players []CollectivePlayer }`; `CollectivePlayer{ PlayerID, TeamID string; Stats map[string]int64 }`.

### Sem novos erros, sem nova RPC, sem store

Reusa `ErrInvalidOutcome`/`ErrUnsupportedOutcome`/`ErrSessionNotFound`. Sessão em memória. Sem migration.

## Casos de Teste Obrigatórios

- [ ] (service) `ReportResult` com `collective` válido (SUCCESS) → carimba `team_id`, publica `kind=collective`, encerra a sessão (`CloseResult`)
- [ ] (service) `collective` com `players` vazio → `ErrInvalidOutcome`; `outcome` fora do vocabulário → `ErrInvalidOutcome`; cada valor válido (SUCCESS/FAILURE) → aceito
- [ ] (service) carimbo: jogador recebe o `team-1` da formação; sobrescreve o que o server ecoou
- [ ] (service) `collective` para `matchId` inexistente → `ErrSessionNotFound`
- [ ] (service) `win_loss_draw`/`placement`/`participation_event` seguem sem regressão
- [ ] (publisher) envelope `collective` serializa `kind`/`matchId`/`gameId`/`outcome`/`players[]` (SNS fake); Log fallback loga o desfecho + jogadores
- [ ] (transport) `toMatchOutcome` mapeia o `oneof` `collective` → `domain` (outcome/players/stats); `oneof` vazio → `ErrUnsupportedOutcome`

## Fora do escopo

- **Sinks** (conquistas de missão, leaderboard `missions_completed`/stats, analytics de sucesso/falha) — fatia **C3**.
- **Console** (surfacing de taxa de sucesso) — fatia **C4**.
- **Missão/dificuldade carregadas na sessão, bot-fill, join-in-progress** — Aberturas do roadmap.
