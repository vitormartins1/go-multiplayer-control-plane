# Spec: 028-session-closed-event

**Serviço:** session-service (fatias em analytics-service, admin-bff, admin-ui)
**Status:** Done
**Data:** 2026-07-13
**Parte de:** evolução do Analytics (item C da ordem A→B→C→D decidida com o
usuário) — "session health": % de sessões abandonadas, duração média de um
mundo/partida.

## Contexto

`closeSession` (session_service.go:482) hoje só persiste o fechamento no
próprio Postgres do session-service (`closed_sessions`, spec 025/027) —
**nenhum evento é publicado no fan-out quando a sessão fecha**, seja qual
for o motivo. Os únicos eventos que o analytics-service recebe hoje
(`win_loss_draw`/`placement`/`collective`/`participation_event`) descrevem
o RESULTADO de uma partida ou um evento de participação — nenhum descreve
o CICLO DE VIDA da sessão em si (quanto tempo durou, por que fechou).

Efeito: hoje é impossível responder, pro Analytics, perguntas como "que %
das sessões de host_world terminam abandonadas (crash) vs encerradas pelo
próprio host?" ou "qual a duração média de uma partida/mundo?" — o dado
existe (`closed_sessions.opened_at`/`closed_at`/`close_reason` no
session-service), mas nunca atravessa pro analytics-service.

Confirmado: `closeSession` é chamado uniformemente pra TODOS os arquétipos
(TTL sweeper, admin, `ReportResult`, `host_left`, `HOST_KILLED`) — um único
evento novo no fechamento cobre os 4 arquétipos de uma vez, sem precisar de
4 specs separadas (diferente de `win_loss_draw`/`placement`/`collective`,
que são polimórficos por natureza do RESULTADO, não do fechamento).

## Comportamento

- WHEN `closeSession` termina de persistir o histórico (`persistHistory`)
  -> THE SYSTEM SHALL publicar um evento `session_closed` no tópico
  match-results (mesmo tópico dos demais kinds — ADR-0020), best-effort:
  falha só loga + incrementa `session_closed_publish_failures_total`,
  **nunca** propaga erro pro chamador (mesma filosofia do `persistHistory`,
  spec 025 — o fechamento da sessão não pode depender de um destino
  externo).
- O evento é publicado **pra toda sessão fechada, qualquer motivo**
  (`result`/`ttl`/`admin`/`abandoned`/`host_left`/`host_killed`) e
  **qualquer arquétipo** (`queue_arena`/`host_world_dropin`/
  `battle_royale`/`coop_mission`) — um único kind novo, sem variação por
  arquétipo (diferente de `win_loss_draw`/`placement`/`collective`: aqui o
  dado publicado — duração, motivo — é o MESMO shape pra todos).
- O envelope carrega `sessionId` (chave de idempotência no
  analytics-service, mesmo padrão de `matchId`/`eventId` dos outros kinds),
  `gameId`, `modeId`, `archetype` (extraído de `sess.Profile.Archetype`,
  mesmo padrão já usado em `postgres_history.go` — necessário porque, ao
  contrário dos outros kinds, o `kind=session_closed` sozinho não diz de
  qual arquétipo a sessão é), `closeReason`, `openedAt`, `closedAt` (o
  analytics-service computa a duração a partir dos dois, não o
  session-service — mesmo espírito de não pré-computar o que o consumidor
  pode derivar).
- `WithSessionClosedPublisher` (Option nova) — nil-safe, mesmo padrão de
  `WithHistoryStore`: sem publisher configurado, `closeSession` não tenta
  publicar (dev sem fan-out configurado continua funcionando).

## Contratos de Interface

### Domínio (session-service)
```go
// SessionClosedPublisher é a Port de saída do fan-out do fechamento de
// sessão (spec 028) — alimenta o "session health" do analytics-service.
// Publica no MESMO tópico match-results, kind=session_closed. Best-effort:
// closeSession não falha se isso falhar (mesma filosofia de HistoryStore).
type SessionClosedPublisher interface {
    PublishSessionClosed(ctx context.Context, sess *Session) error
}
```

### Envelope (tópico match-results, mesmo padrão dos outros kinds)
```go
type sessionClosedEnvelope struct {
    SchemaVersion int    `json:"schemaVersion"`
    Kind          string `json:"kind"` // "session_closed"
    SessionID     string `json:"sessionId"`
    GameID        string `json:"gameId"`
    ModeID        string `json:"modeId,omitempty"`
    Archetype     string `json:"archetype,omitempty"`
    CloseReason   string `json:"closeReason"`
    OpenedAt      string `json:"openedAt"` // RFC3339
    ClosedAt      string `json:"closedAt"` // RFC3339
}
```

## Critérios de Aceite

1. Sessão `queue_arena` fechada por `ReportResult` -> evento
   `session_closed` publicado com `closeReason=result`, `archetype=
   queue_arena`, timestamps corretos.
2. Sessão `host_world_dropin` fechada por TTL, admin, `host_left` ou
   `HOST_KILLED` -> evento publicado com o `closeReason` correspondente
   (cobre os 4 motivos que HOJE nunca geram nenhum evento no fan-out).
3. Publisher não configurado (dev) -> `closeSession` funciona normalmente,
   sem tentar publicar, sem erro.
4. Falha ao publicar (ex.: SNS fora) -> `closeSession` completa normalmente
   (sessão fecha, histórico persiste), só loga + incrementa a métrica.
5. Achievements-service e leaderboard-service (outros consumidores do MESMO
   tópico) -> ignoram o `kind=session_closed` sem erro (comportamento já
   existente pra kind desconhecido, confirmado no código: "envelope
   ignorado (kind não tratado)").

## Fora de escopo

- **O que o analytics-service faz com o evento** — spec própria
  (analytics-service, próxima desta cadeia).
- **Duração por participante** dentro de uma sessão host_world (quem ficou
  quanto tempo) — a duração aqui é da SESSÃO INTEIRA (openedAt→closedAt),
  não por jogador (mesmo escopo já recusado na spec 027).
- **Reprocessamento de sessões fechadas ANTES desta spec** — não há
  backfill; o "session health" só existe a partir de sessões fechadas
  depois do deploy desta mudança (mesmo padrão de toda dimensão nova
  adicionada neste projeto — dado legado fica de fora, sem migration
  retroativa).
