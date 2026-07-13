# Spec: 013-session-health

**Serviço:** analytics-service (fatia em admin-bff)
**Status:** Done
**Data:** 2026-07-13
**Parte de:** evolução do Analytics (item C) — consome o evento
`session_closed` do session-service (spec 028) pra responder "que % das
sessões terminam abandonadas?" e "qual a duração média de uma partida/mundo?".

## Contexto

O session-service (spec 028) passa a publicar um evento `session_closed`
sempre que QUALQUER sessão fecha, qualquer motivo, qualquer arquétipo —
carregando `sessionId`/`gameId`/`modeId`/`archetype`/`closeReason`/
`openedAt`/`closedAt`. O analytics-service ainda não tem nenhum consumidor
pra esse `kind` (é novo). Esta spec adiciona o quarto "tipo de job" ao
worker pool (junto de match/participation/placement/collective) e um
rollup novo que responde as duas perguntas que motivaram o item C da
evolução do Analytics.

## Comportamento

- WHEN o consumidor recebe um envelope `kind=session_closed` -> THE SYSTEM
  SHALL desserializar e submeter ao worker pool (mesmo padrão de
  fan-in/fan-out dos outros 3 kinds).
- WHEN o pool agrega o evento -> THE SYSTEM SHALL: (1) marcar `sessionId`
  como processado (idempotência, mesmo padrão de `processed_matches`
  por `matchId`/`processed_participations` por `eventId`); (2) se já
  processado, no-op idempotente; (3) senão, incrementar o rollup
  `(game_id, mode_id, archetype, close_reason) -> count, duration_sum_ms`
  numa única transação.
- `GameAnalytics` (porta já existente, ganhou `modeID` na spec 012) SHALL
  devolver também `SessionHealth []SessionCloseStat` — a distribuição por
  `closeReason`, JÁ filtrada por `modeID` quando aplicável (esta tabela
  nasce com `mode_id`, sem a lacuna do item D que afeta as tabelas mais
  antigas).
- Não há uma métrica pré-computada de "% abandonada" — a distribuição crua
  por `closeReason` (contagem + duração) é o que a API expõe; a
  interpretação ("isso conta como abandono?") é decisão de apresentação
  (admin-ui), não do analytics-service — mesmo espírito de `RoleParticipations`/
  `StatSummary` (dado cru, sem rótulo de negócio embutido).

## Contratos de Interface

### Domínio (analytics-service)
```go
// SessionCloseEvent é o fechamento de uma sessão, qualquer arquétipo/motivo
// (session-service/028, kind=session_closed). Unidade de trabalho do
// caminho de "session health" do pool.
type SessionCloseEvent struct {
    SessionID   string
    GameID      string
    ModeID      string
    Archetype   string
    CloseReason string
    OpenedAt    time.Time
    ClosedAt    time.Time
}

// SessionCloseStat é a distribuição agregada por motivo de fechamento —
// conta + duração total (Avg = DurationSumMs / Count, computado no read,
// mesmo padrão de StatSummary).
type SessionCloseStat struct {
    ModeID        string
    Archetype     string
    CloseReason   string
    Count         int64
    DurationSumMs int64
    AvgDurationMs float64
}

type MatchRepository interface {
    // ... métodos existentes
    // ApplySessionClosed agrega um fechamento de sessão (spec 013),
    // idempotente por sessionID (processed_sessions) numa tx. dup=true se
    // já processado (no-op).
    ApplySessionClosed(ctx context.Context, evt SessionCloseEvent) (dup bool, err error)
}

type Ingestor interface {
    // ... métodos existentes
    SubmitSessionClosed(ctx context.Context, evt SessionCloseEvent) <-chan error
}

type GameAnalytics struct {
    // ... campos existentes (spec 012)
    SessionHealth []SessionCloseStat
}
```

### Schema (migration 010 do analytics-postgres)
```sql
CREATE TABLE processed_sessions (
    session_id   TEXT PRIMARY KEY,
    game_id      TEXT NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE game_session_rollup (
    game_id         TEXT NOT NULL,
    mode_id         TEXT NOT NULL DEFAULT '',
    archetype       TEXT NOT NULL DEFAULT '',
    close_reason    TEXT NOT NULL,
    count           BIGINT NOT NULL DEFAULT 0,
    duration_sum_ms BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (game_id, mode_id, close_reason)
);
```

### HTTP admin (aditivo)
```json
{
  "sessionHealth": [
    {"modeId": "mundo-vivo", "archetype": "host_world_dropin", "closeReason": "host_left", "count": 12, "durationSumMs": 4320000, "avgDurationMs": 360000.0},
    {"modeId": "mundo-vivo", "archetype": "host_world_dropin", "closeReason": "ttl", "count": 3, "durationSumMs": 5400000, "avgDurationMs": 1800000.0}
  ]
}
```

## Critérios de Aceite

1. Evento `session_closed` real (`closeReason=result`, `queue_arena`) ->
   `game_session_rollup` grava 1 linha, `count=1`.
2. Reentrega do mesmo `sessionId` (SQS at-least-once) -> no-op, `count`
   não dobra.
3. `?modeId=` filtra `SessionHealth` corretamente (mesma mecânica da spec
   012 — tabela já nasce com `mode_id`).
4. Dois motivos de fechamento diferentes no mesmo modo -> duas linhas
   distintas em `SessionHealth`, cada uma com sua própria contagem/duração.
5. Validado com dado real: fechar sessões reais dos 4 arquétipos por
   motivos diferentes, conferir `sessionHealth` refletindo cada uma.

## Fora de escopo

- **Interpretação de "abandono"** (agrupar `ttl`+`abandoned` como "sem
  conclusão" etc.) — decisão de apresentação, cabe ao admin-ui quando
  desenhar o widget, não a esta spec.
- **Série diária de fechamentos** ("sessões fechadas por dia") — só o
  rollup cumulativo por enquanto; se houver demanda real por uma série
  temporal aqui, vira spec própria (mesmo padrão de adiamento já usado
  noutras partes do Analytics).
- **Duração por participante** — já fora de escopo na origem (spec 028 do
  session-service).
