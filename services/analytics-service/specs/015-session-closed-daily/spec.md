# Spec: 015-session-closed-daily

**Serviço:** analytics-service (fatias em admin-bff, admin-ui)
**Status:** Done (implementada e mergeada; status corrigido na revisão 2026-09)
**Data:** 2026-07-13
**Parte de:** fecha o follow-up "fora de escopo" explícito da spec 013 —
série diária de sessões fechadas, além do rollup cumulativo.

## Contexto

`game_session_rollup` (spec 013) agrega fechamentos de sessão CUMULATIVOS
por `(game_id, mode_id, close_reason)` — não há como ver a EVOLUÇÃO no
tempo (ex.: "as sessões abandonadas aumentaram essa semana?"). As 4 séries
diárias que já existem (`game_daily`, `game_placement_daily`,
`game_mission_daily`, `game_participation_daily`) seguem o mesmo padrão de
schema (`game_id, mode_id, day, <contagem>`); replicar esse padrão aqui é
mecânico.

## Comportamento

- WHEN `ApplySessionClosed` agrega um fechamento (idempotente, já
  existente) -> THE SYSTEM SHALL também incrementar
  `game_session_daily(game_id, mode_id, day, count)` na MESMA transação.
- `GameAnalytics` SHALL ganhar `SessionCloseDaily []DailyPoint` — série do
  total de sessões fechadas por dia (todos os motivos somados; motivo
  fica só no rollup cumulativo, não duplicado aqui — mesmo espírito de
  `game_daily`, que também não quebra por resultado).
- Filtrável por `modeID` (mesmo mecanismo `dailySeries` já usado nas
  outras 3 séries — reaproveitado, não reimplementado).

## Contratos de Interface

### Schema (migration 012 do analytics-postgres)
```sql
CREATE TABLE game_session_daily (
    game_id TEXT NOT NULL,
    mode_id TEXT NOT NULL DEFAULT '',
    day     DATE NOT NULL,
    count   BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (game_id, mode_id, day)
);
```

### Domínio
```go
type GameAnalytics struct {
    // ... campos existentes
    SessionCloseDaily []DailyPoint // sessões fechadas por dia, todos os motivos somados
}
```

### HTTP admin (aditivo)
```json
{"sessionCloseDaily": [{"day": "2026-07-13", "matches": 4}]}
```
(campo `matches` reaproveita `dailyDTO` — mesmo shape das outras 3 séries,
nome do campo já genérico o bastante pra não precisar de um DTO novo.)

## Critérios de Aceite

1. Fechar 3 sessões em dias diferentes -> `sessionCloseDaily` com 3 pontos,
   1 cada.
2. Reentrega (SQS at-least-once) -> não dobra a contagem do dia (mesma
   idempotência de `processed_sessions` já usada pro rollup).
3. `?modeId=` filtra a série corretamente.
4. Regressão zero: `SessionHealth` (rollup cumulativo) continua idêntico.

## Fora de escopo

- Quebrar a série diária por `closeReason` (série empilhada) — só o total
  por dia; o detalhe por motivo já existe no rollup cumulativo
  (`SessionHealth`). Se houver demanda real por "abandono ao longo do
  tempo" especificamente, é spec nova.
