# Spec: 008-stat-criteria

**Serviço:** achievements-service (+ cauda admin-bff/admin-ui)
**Status:** Draft
**Data:** 2026-07-06
**ADR:** 0023 (resultado polimórfico — `stats` por jogador). É a fatia **A1** do roadmap `queue_arena`.

## Contexto

O outcome da partida já carrega `stats` por jogador (`map[string]int64`, ex.: `{"kills":7}` — spec session/009, no envelope do fan-out). Hoje o catálogo só tem critério **por evento** (`event_type` + `target` = nº de ocorrências: `match_played`/`match_won`/`match_lost`/`match_drawn`). Esta fatia adiciona um segundo tipo de critério: **por métrica (stat)**, ex.: "10 kills acumulados" ou "5 kills numa partida". O consumer lê `stats` e avança o progresso pela **métrica** (não por contagem de eventos), mantendo a idempotência por `matchID` (ADR-0020).

`teamId` (conquistas condicionadas a time) fica **fora desta fatia** (follow-up — ver Fora do escopo).

## Comportamento

### Tipos de critério

Uma conquista tem **um** critério, discriminado por `statKey`:

- `statKey == ""` → **critério por evento** (comportamento atual): `eventType` obrigatório; `target` = nº de ocorrências.
- `statKey != ""` → **critério por stat**: `eventType` deve estar **vazio**; `target` = **threshold** da métrica; `statMode ∈ {cumulative, per_match}`.
  - `cumulative` → soma o valor do stat **entre partidas**; desbloqueia quando o acumulado atinge `target`.
  - `per_match` → considera o valor do stat **de uma única partida**; desbloqueia quando **uma** partida atinge `target`. O `progress` guarda o **melhor** valor de partida (capado em `target`), para exibir o quão perto chegou.

### Aplicar o resultado (consumer/fan-out)

- WHEN o fan-out entrega uma partida → THE SYSTEM SHALL, por jogador, além dos eventos do outcome (inalterado), avançar as conquistas por stat cujo `statKey` casa uma chave do `stats` do jogador:
  - `cumulative`: `progress = LEAST(progress + valor, target)`; desbloqueia ao alcançar `target`.
  - `per_match`: `progress = LEAST(GREATEST(progress, valor), target)`; desbloqueia se `valor >= target`.
- THE SYSTEM SHALL manter a **idempotência por matchID**: catálogo + progresso + marca `processed_matches` na **mesma transação**; reprocessar a partida é no-op (o `ZINCRBY`-like fica seguro porque a tx só roda uma vez por matchID).
- IF um `statKey` do catálogo não aparece no `stats` do jogador → nenhuma mudança para aquela conquista (não cria progresso).

### Catálogo (CRUD, admin)

- `CreateAchievement`/`UpdateAchievement` aceitam `statKey`/`statMode`. Validação:
  - `statKey != ""` → deve casar `^[a-z][a-z0-9_]*$`; `statMode ∈ {cumulative, per_match}`; `eventType` vazio; `target >= 1`. Caso contrário → `ErrInvalidAchievement`.
  - `statKey == ""` → `eventType` obrigatório (como hoje).
- O catálogo lido (`ListCatalog`/admin) expõe `statKey`/`statMode` (vazios nas conquistas por evento).

## Contratos de Interface

- **Domain:** `Achievement` +`StatKey string` +`StatMode string`. `MatchResult` +`Stats map[string]int64`. `PlayerEvents` → renomeado **`PlayerMatchInput`** (`PlayerID`, `EventTypes []string`, `Stats map[string]int64`).
- **Repository:** `FindByStatKey(ctx, gameID, statKey) ([]Achievement, error)`; `ApplyMatchResult` passa a receber `[]PlayerMatchInput` e, na tx, aplica eventos (como hoje) + stats (nova SQL por modo). `ListCatalog`/`FindByEventType`/scan incluem `stat_key`/`stat_mode`.
- **Service:** `ApplyMatchResult` monta `PlayerMatchInput` com `Stats`; `Create/UpdateAchievement` validam o critério.
- **Consumer** (`cmd/consumer`): o envelope ganha `stats map[string]int64` por resultado; repassa no `MatchResult.Stats`.
- **Migration `005_stat_criteria.sql`:** `achievements` +`stat_key TEXT NOT NULL DEFAULT ''` +`stat_mode TEXT NOT NULL DEFAULT ''`; índice parcial `(game_id, stat_key) WHERE stat_key <> ''`. Seed de exemplo no `sandbox`.
- **admin-bff / admin-ui:** cauda — propagar `statKey`/`statMode` (adapter + editor com toggle de tipo de critério).

## Casos de Teste Obrigatórios

- [ ] (service) create stat válido (`kills`/`cumulative`/`target 10`) → ok; `statKey` inválido, `statMode` inválido, `eventType` junto com `statKey` → `ErrInvalidAchievement`
- [ ] (service) `ApplyMatchResult` propaga `Stats` no `PlayerMatchInput`
- [ ] (repo, integração) `cumulative`: duas partidas somam (3 + 8 kills, target 10 → desbloqueia na 2ª); `per_match`: 4 depois 6 (target 5 → desbloqueia na 2ª, progress = 5); stat ausente → sem progresso; idempotência: reprocessar matchID = no-op — testcontainers
- [ ] (consumer) desserializa `stats` e repassa
- [ ] critério por evento **inalterado** (regressão)

## Fora do escopo

- **`teamId`** — conquistas por time / "vencer como time X" (follow-up; o outcome já carrega `teamId`).
- **leaderboard acumulativo** (A2, spec própria).
- Presets/seed por jogo real (os stats vivem no `queue_arena`; seed só no `sandbox`).

## Follow-up — delta negativo clampado a 0

Achado no `/code-review` do projeto como um todo (2026-07-10): `stats` chega
via envelope SQS de um dedicated server (fonte externa, não confiável) e o
`delta` do stat era somado direto no SQL sem validar sinal — a spec original
(§ "Aplicar o resultado") assumia `valor >= 0` sem enforçar. Um payload
malformado com `kills: -999` no modo `cumulative` decrementaria (ou zeraria
via `LEAST`) progresso já acumulado legitimamente; no INSERT inicial (jogador
sem linha ainda), gravaria `progress` negativo em qualquer modo.

`recordStatProgress` (`internal/repository/postgres_achievement.go`) agora
clampa `delta` a 0 antes de montar a query — ponto único, cobre os dois modos
e o INSERT inicial. Sem rejeitar a mensagem inteira (evitaria poison-message
no SQS: um envelope permanentemente malformado nunca marcaria
`processed_matches`, causando redelivery infinito). Testes novos:
`TestApplyMatchResult_StatCumulative_DeltaNegativo_NaoDecrementa` e
`TestApplyMatchResult_StatCumulative_DeltaNegativoNaPrimeiraPartida_NaoGravaNegativo`.

## Follow-up — N+1 queries em ApplyMatchResult/ApplyParticipationEvent

Achado no `/code-review` do projeto como um todo (2026-07-10): `ApplyMatchResult`
rodava `findByEventType`/`findByStatKey` **dentro** do loop por jogador ×
event type/stat — numa partida com P jogadores e E/S critérios cada, isso é
P×(E+S) SELECTs sequenciais dentro da mesma transação (mais lock time,
round-trips seriais ao Postgres). `ApplyParticipationEvent` tinha o mesmo
padrão (sem o loop por jogador, mas ainda uma query por event type/stat).

Adicionadas `findByEventTypes`/`findByStatKeys` (uma query com
`= ANY($2::text[])`, agrupando o resultado por chave em Go) e
`distinctEventTypes`/`distinctStatKeys` (agregam as chaves de todos os
jogadores antes do batch). As duas funções agora buscam o catálogo casado
**uma vez por transação**, independente do número de jogadores. Teste novo
`TestApplyMatchResult_MultiJogadorMultiCriterio_CadaUmSoRecebeOSeu` prova que
o agrupamento em memória continua atribuindo o progresso ao jogador certo
(p2 não herda o stat que só p1 reportou).
