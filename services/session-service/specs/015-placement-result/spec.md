# Spec: 015-placement-result

**Serviço:** session-service
**Status:** Rascunho (aguardando aprovação)
**Data:** 2026-07-07
**ADR:** 0023 (perfil de multiplayer por jogo) — arquétipo `battle_royale`
**Roadmap:** `docs/game-models/battle_royale-roadmap.md` — fatia **BR2** (o núcleo)
**Depende de:** BR1 (`matchmaking/008` — lobby/squads) · reusa o `ReportResult` e o fan-out por `kind` (specs `009`/`013`, ADR-0020)

## Contexto

Terceira fatia do `battle_royale` (ficha `docs/game-models/03-battle-royale.md`) e o **núcleo** do arquétipo: o resultado por **colocação ordinal** (1º..Nº). Hoje o `MatchOutcome` só tem o kind `win_loss_draw` (`session/009`); o fan-out o publica no tópico `match-results` com um envelope discriminado por `kind` (spec `013` já provou o padrão com `participation_event`).

Esta fatia adiciona o kind **`placement`**: o server do BR reporta as colocações de **todos** no fim (resultado **terminal**, um `ReportResult` — decisão #1 do roadmap; a eliminação progressiva é gameplay do data plane), e o fan-out o distribui. Fecha o "gap central" (colocação ordinal) para este arquétipo. Os **sinks** que reagem ao `placement` (conquistas Top-N, leaderboard, analytics) são a fatia **BR3** — aqui só produzimos e publicamos.

**Fronteira (ficha §6):** zona/loot/spawn/o momento físico da eliminação = data plane. O control plane recebe só as colocações **finais**; o `respawnMode` (que decide *quando* a colocação é fixada) é honrado pelo server, não pelo session.

## Comportamento

### Reportar o resultado por colocação

- WHEN o server chama `ReportResult(matchId, placement{...})` com um outcome `placement` **válido** → THE SYSTEM SHALL carimbar o `team_id` canônico de cada entrada (a partir da composição registrada na formação — squad `squad-K` da BR1), **publicar** o resultado no `match-results` com envelope `kind=placement`, e **encerrar** a sessão (`CloseResult`) — exatamente o fluxo do `win_loss_draw`.
- IF não houver sessão para o `matchId` → `ErrSessionNotFound` (inalterado).
- THE SYSTEM SHALL manter o `win_loss_draw` (queue_arena) e o `participation_event` (host_world_dropin) **sem regressão**.

### Forma do outcome `placement`

Um `placement` carrega uma entrada por participante:

| Campo | Tipo | Significado |
|---|---|---|
| `playerId` | string | jogador |
| `teamId` | string | squad (`squad-K`); vazio em solo/FFA — **carimbado pelo session** a partir da formação, não confiado no que o server ecoa |
| `placement` | int | colocação ordinal (1 = vencedor); squads compartilham a colocação do time |
| `survivedMs` | int64 | tempo de sobrevivência (ms); intrínseco ao BR |
| `stats` | map[string]int64 | métricas extras opcionais (ex.: `kills`); pode ser vazia |

### Validação (estrutural, server é autoritativo)

Como no `win_loss_draw`, a validação é **estrutural** — o game server é autoritativo sobre o resultado; o session **não** valida contra o perfil do jogo nem recomputa a corrida:

- IF `entries` vazio → `ErrInvalidOutcome`.
- IF alguma entrada tem `placement < 1` → `ErrInvalidOutcome`.
- IF alguma entrada tem `survivedMs < 0` → `ErrInvalidOutcome`.
- THE SYSTEM SHALL **aceitar** empates de colocação (co-eliminação — ficha §5) e **não** exigir colocações contíguas (1..N sem buracos) nem que squads compartilhem a colocação — coerência de ranking é responsabilidade do data plane (mesma filosofia do `win_loss_draw`, que não valida "quem venceu").
- IF nenhum campo do `oneof` estiver setado → `ErrUnsupportedOutcome` (inalterado).

### Carimbo de time (reuso)

- WHEN o outcome é `placement` → THE SYSTEM SHALL sobrescrever o `teamId` de cada entrada com o time registrado na formação (`TeamByPlayer`); jogador fora da composição (solo/FFA) fica com `teamId` vazio. É o mesmo comportamento do `stampTeams` do `win_loss_draw`, estendido ao ponteiro `Placement`.

### Fan-out (reuso do envelope por `kind`)

- THE SYSTEM SHALL publicar no **mesmo** tópico `match-results` um envelope `kind=placement` com `schemaVersion`, `matchId`, `gameId` e `entries[]` (`playerId`/`teamId`/`placement`/`survivedMs`/`stats`). Reusa o mecanismo (SNS real + Log fallback); só o **payload** por kind é novo. Consumidores atuais ignoram o que não entendem; os sinks do BR (BR3) reagem.

## Contratos de Interface

### gRPC (`session.proto`) — extensão do `oneof`

- `ReportResultRequest.outcome` (oneof) ganha `PlacementResult placement = 3;`.
- Novas mensagens `PlacementResult{ repeated PlacementEntry entries = 1; }` e `PlacementEntry{ player_id, team_id, placement (int32), survived_ms (int64), stats map<string,int64> }`.
- **Regenerar com `buf generate`** (nunca `sed` no `.pb.go` — gotcha do `go_package`).

### Domínio (session) — extensão

- const `OutcomePlacement = "placement"`.
- `MatchOutcome` ganha `Placement *PlacementResult` (tagged union por ponteiro; exatamente o ponteiro do `Kind` é não-nil).
- `PlacementResult{ Entries []PlacementEntry }`; `PlacementEntry{ PlayerID, TeamID string; Placement int; SurvivedMs int64; Stats map[string]int64 }`.

### Sem novos erros, sem nova RPC, sem store

Reusa `ErrInvalidOutcome`/`ErrUnsupportedOutcome`/`ErrSessionNotFound`. Sessão continua em memória. Sem migration.

## Casos de Teste Obrigatórios

- [ ] (service) `ReportResult` com `placement` válido → carimba `team_id`, publica `kind=placement`, encerra a sessão (`CloseResult`)
- [ ] (service) `placement` com `entries` vazio → `ErrInvalidOutcome`; `placement < 1` → `ErrInvalidOutcome`; `survivedMs < 0` → `ErrInvalidOutcome`
- [ ] (service) `placement` com empate de colocação (dois `placement=1`) → **aceito**
- [ ] (service) carimbo: entrada de squad recebe o `squad-K` da formação; solo/FFA fica com `teamId` vazio (sobrescreve o que o server ecoou)
- [ ] (service) `placement` para `matchId` inexistente → `ErrSessionNotFound`
- [ ] (service) `win_loss_draw` e `participation_event` seguem sem regressão
- [ ] (publisher) envelope `placement` serializa `kind`/`matchId`/`gameId`/`entries[]` corretamente (SNS fake); Log fallback loga as entradas
- [ ] (transport) `toMatchOutcome` mapeia o `oneof` `placement` → `domain` (entries/placement/survivedMs/stats); `oneof` vazio → `ErrUnsupportedOutcome`

## Fora do escopo

- **Sinks** (conquistas Top-N, leaderboard por colocação/kills, analytics de distribuição) — fatia **BR3**.
- **Eliminação progressiva / tracking ao vivo** (Rooms mostrando vivos/eliminados) — follow-up (decisão #1); aqui o resultado é **terminal**.
- **Colocação provisória** (respawn `arena_1v1`/`reboot_timer` mediado pelo backend) — data plane (decisão #2).
- **Console** (surfacing da colocação) — fatia BR4.
