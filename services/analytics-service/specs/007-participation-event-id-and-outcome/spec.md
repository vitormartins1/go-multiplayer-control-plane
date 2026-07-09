# Spec: 007-participation-event-id-and-outcome

**Serviço:** analytics-service
**Status:** Done (2026-07-09)
**Data:** 2026-07-09
**ADR:** 0023 — **arquétipo `host_world_dropin`, follow-ups #4 e #5** (lado analytics: idempotência por `eventId` + dimensão `outcome`)
**Roadmap:** `docs/game-models/host_world_dropin-follow-ups.md` → itens 4 e 5
**Depende de:** `session-service/specs/021-participation-event-id` (o `eventId` precisa existir no envelope)
**Emparelhada com:** `achievements-service/specs/012-participation-event-id` (mesma migração de idempotência, serviço irmão)

## Contexto

Dois follow-ups do mesmo arquétipo tocam a **mesma tabela** (`game_participation_rollup`) e por isso entram juntos nesta spec:

1. **Follow-up #4 (eventId):** `processed_participations` (migration 003, própria cópia deste serviço) dedupe por `(session_id, player_id, event)` — mesma limitação do achievements (spec irmã `012`): o boss que respawna (spec 004 do Unity) faz o segundo `BOSS_DEFEATED` na mesma sessão ser descartado.
2. **Follow-up #5 (outcome):** `game_participation_rollup` agrega por `(game_id, role, event)` — **sem** `outcome`. Não dá pra distinguir "invasão bem-sucedida" (`HOST_KILLED`·`SUCCESS`) de "invasão fracassada" (`DIED`·`FAIL` do invasor) no funil — na prática, hoje o rollup já separa isso **por tipo de evento** (`HOST_KILLED` só existe com outcome=SUCCESS na prática atual, `DIED` só com FAIL), mas a spec 004 introduz o caso `HOST_KILLED`·`FAIL` (host morto pelo boss, sem invasor — ver spec 004 do Unity, "Fora de escopo" original revisado) — **sem a dimensão outcome, esse caso fica indistinguível de um `HOST_KILLED` normal no rollup**, inflando a taxa de sucesso de invasão silenciosamente.

## Comportamento

### `eventId` (follow-up #4)

- Mesmo comportamento da spec irmã `achievements/012`: dedupe por `eventId` quando presente; fallback determinístico `(session_id, player_id, event)` quando vazio (data plane antigo).

### `outcome` no rollup (follow-up #5)

- WHEN um `participation_event` é aplicado → THE SYSTEM SHALL agregar `game_participation_rollup` por `(game_id, role, event, outcome)` — `outcome` vira parte da chave, não só um campo solto.
- WHEN o console/admin-bff lê `RoleParticipations` → THE SYSTEM SHALL devolver `outcome` em cada linha (o consumidor decide como exibir — ex.: "HOST_KILLED: 12 SUCCESS / 2 FAIL").
- **Dados existentes:** a migration preserva os totais já agregados, atribuindo `outcome=''` (bucket "desconhecido/anterior à dimensão") — não há como reconstruir o outcome retroativo dos eventos já consumidos (só o total por `(game,role,event)` sobreviveu, não os eventos individuais). Aditivo daqui pra frente: todo evento novo grava com o `outcome` real.

## Contratos de Interface

### Consumer (`internal/consumer/consumer.go`)

- `parseParticipation`: o `struct` local de parsing ganha `EventID string \`json:"eventId"\``.

### Domínio

- `ParticipationEvent` (domain) += `EventID string`.
- `ParticipationStat` (domain) += `Outcome string`.

### Repository

- `ApplyParticipation`: usa `event_id` (com fallback) na tabela `processed_participations`; o `INSERT ... ON CONFLICT` de `game_participation_rollup` passa a incluir `outcome` na chave.
- Leitura (`RoleParticipations`/query do `GameAnalytics`): `SELECT role, event, outcome, count ... ORDER BY role, event, outcome`.

### Erros de domínio

- Nenhum novo.

## Modelo de Dados

**Migration 006** (`006_participation_event_id_and_outcome.sql`), na mesma tabela `processed_participations` deste serviço (cópia própria, banco próprio — mesmo shape da migration do achievements, serviços diferentes):

- `processed_participations`: mesmo backfill/troca de PK da spec irmã (`achievements/012`, D2).
- `game_participation_rollup`: `ADD COLUMN outcome TEXT NOT NULL DEFAULT ''`; trocar a PK de `(game_id, role, event)` pra `(game_id, role, event, outcome)`. Linhas existentes ficam com `outcome=''` (bucket "pré-dimensão", não uma perda de dado — só um agregado que não tinha essa granularidade quando foi somado).

## Fora de escopo

- **Reconstruir o outcome retroativo** dos totais já agregados antes desta migration — impossível sem os eventos individuais (só o `analytics/004` original guardava o total, não o histórico bruto).
- **UI do funil (H3)** consumindo a nova dimensão — fica pro `admin-bff`/`admin-ui`, fora deste serviço.
- **Geração do `eventId`** — vem do `session/021`/Unity.

## Critérios de Aceite

- [x] Dois `BOSS_DEFEATED` do mesmo `(game,session,player)` com `eventId` diferentes → contam **duas vezes** no `participation_events_total`/rollup relacionado.
- [x] `HOST_KILLED`·`SUCCESS` e `HOST_KILLED`·`FAIL` (mesmo jogo/papel/evento, outcome diferente) → linhas **separadas** no rollup, cada uma com sua própria contagem.
- [x] `eventId` vazio → fallback determinístico, comportamento idêntico ao pré-migration.
- [x] Migration não apaga dados existentes; totais antigos aparecem sob `outcome=''`.
- [x] `RoleParticipations`/`GameAnalytics` expõem `outcome` em cada linha.
- [x] Testes com `-race`, cobertura mantida no baseline atual do serviço.
