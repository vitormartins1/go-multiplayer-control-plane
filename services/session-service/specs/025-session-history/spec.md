# Spec: 025-session-history
**Serviço:** session-service (fatias em admin-bff e admin-ui)
**Status:** Done (2026-07-13) — Fatias 1-4 implementadas e validadas (ver tasks.md)
**Data:** 2026-07-12

## Contexto

Sessões são **100% in-memory** no session-service: as ativas num mapa, as
fechadas num ring bounded (`historyLimit=200`). Restart do serviço apaga tudo —
achado ao vivo na bateria de testes do `host_world_dropin` (2026-07-12): o
operador estava explorando as salas no admin console e a lista sumiu no recycle
do serviço. Consequências hoje:

- A aba **Sessions** de um jogo só mostra o que o processo atual viu (e no
  máximo 200 fechadas, globais, não por jogo).
- Não existe **histórico por jogador** em lugar nenhum: a página do player tem
  Achievements/Player data/Cloud saves, mas nenhuma resposta para "quais
  partidas/mundos esse jogador jogou?".
- O pipeline de analytics persiste **rollups agregados** (participações por
  modo etc.), não sessões individuais — não serve de fonte para "a sala X, com
  quem, quando, por quê fechou".

Pedido do usuário (2026-07-12): histórico de sessão persistido, visível em
(a) uma aba **History** na página do player — depois de Achievements, escopada
ao jogo selecionado (a página já é escopada por jogo via presença/ADR-0019) — e
(b) na aba **Sessions** do jogo, listando as partidas fechadas.

Pré-requisito já feito: `Players` de host_world acumula PARTICIPANTES (não
remove em DIED/LEFT — fix 2026-07-12 no caminho dos sinais/sala do admin).

## Comportamento

### Persistência no fechamento (session-service)
- WHEN qualquer sessão fecha (`closeSession`: result, host morto, admin, TTL,
  abandono, `LeaveWorld`/`CloseWorld` do host) -> THE SYSTEM SHALL gravar um
  registro imutável da sessão fechada no store durável (Postgres), incluindo os
  participantes com papel.
- THE SYSTEM SHALL gravar TODOS os arquétipos (o `closeSession` é comum), não
  só host_world — campos específicos de arquétipo são opcionais no registro.
- IF a escrita no store falhar -> THE SYSTEM SHALL logar e seguir o fechamento
  normalmente (release do server NUNCA depende de destino externo — mesma
  filosofia do fan-out de resultados), e incrementar métrica de falha.
- O ring in-memory continua existindo como está (serve o "recém-fechado" sem
  ida ao banco e é o fallback se o Postgres estiver fora).

### Participantes com papel (domínio)
- WHEN um mundo host_world abre -> o host entra como participante `role=host`.
- WHEN `JoinWorld` injeta um phantom -> o jogador entra como participante com o
  papel do join (`coop`/`invader`); re-entrada não duplica (mantém o papel da
  primeira entrada; se re-entrar com papel diferente, registra a ocorrência mais
  recente).
- WHEN uma sessão de match (queue_arena etc.) é criada -> os jogadores da
  escalação entram como participantes com papel vazio (ou `team:<id>` quando
  houver times).
- Participantes NUNCA são removidos por DIED/LEFT (participação é histórica;
  capacidade/presença continua nos RoleSlots).

### Consulta — sessões fechadas por jogo
- WHEN o operador abre a aba Sessions de um jogo -> THE SYSTEM SHALL listar as
  fechadas do STORE (não mais só do ring), da mais recente para a mais antiga,
  paginadas (`limit` + cursor por `closed_at`), com: sessionId, matchId, modeId,
  origin, closeReason, openedAt/closedAt, participantes (id+papel).
- Sessões ativas continuam vindo do mapa in-memory (fonte de verdade do "agora").

### Consulta — histórico por jogador
- WHEN o operador abre a aba History de um player com um jogo selecionado ->
  THE SYSTEM SHALL listar as sessões FECHADAS daquele jogo em que o player
  participou, da mais recente para a mais antiga, paginadas, cada uma com o
  papel do player naquela sessão, closeReason e duração.
- IF o player não tem sessões no jogo -> lista vazia (estado "sem histórico"),
  sem erro.

### Admin console (admin-ui)
- Página do player: nova aba **History** logo após Achievements, usando o
  seletor de jogo existente da página; colunas: quando (closedAt), modo,
  papel do player, motivo do fechamento, duração, participantes (contagem com
  tooltip de handles).
- Aba Sessions do jogo (`GameSessions`/`Rooms`): as fechadas passam a vir do
  endpoint persistido com paginação ("carregar mais"); visual atual mantido.

## Contratos de Interface

### Service (Ports novas) — session-service
```go
// Port de saída (store durável; adapter Postgres)
type HistoryStore interface {
    SaveClosed(ctx context.Context, sess *domain.Session) error
    ClosedByGame(ctx context.Context, gameID string, limit int, before time.Time) ([]domain.Session, error)
    ClosedByPlayer(ctx context.Context, gameID, playerID string, limit int, before time.Time) ([]domain.Session, error)
}
```

### HTTP admin (session-service; guard de internal key como as demais)
- `GET /admin/games/{gameId}/sessions?state=closed&limit=&before=` — fechadas
  do store, paginadas. (Sem `state=closed`: comportamento atual — ativas do
  mapa + fechadas recentes.)
- `GET /admin/players/{playerId}/sessions?gameId=&limit=&before=` — novo.

### admin-bff (passthrough + enriquecimento)
- `GET /api/games/{gameId}/sessions?state=closed&...` — proxy.
- `GET /api/players/{playerId}/sessions?gameId=&...` — proxy + handles via
  identity (mesmo enriquecimento das salas).

### Domínio
```go
type Participant struct {
    PlayerID string
    Role     string // "host" | "coop" | "invader" | "" (match) | futuro "team:<id>"
    JoinedAt time.Time
}
// Session ganha Participants []Participant (Players []string vira derivado/compat).
```

## Critérios de Aceite

1. Fechou sessão (qualquer motivo, qualquer arquétipo) → registro no Postgres
   com participantes+papéis; falha de escrita não impede o fechamento.
2. Restart do session-service → aba Sessions do jogo continua mostrando as
   fechadas (do store); só as ATIVAS do processo morto somem (viram
   `abandoned` pelo reconcile quando aplicável).
3. Aba History do player mostra as sessões do jogo selecionado com papel/motivo/
   duração; player sem histórico → estado vazio.
4. Coop que morreu num mundo host_world aparece no histórico daquele mundo com
   `role=coop` (integração com o fix de participantes de 2026-07-12).
5. Paginação estável (cursor por `closed_at`) nas duas consultas.
6. Testes: unit do serviço (SaveClosed chamado no close, falha não propaga),
   integração do adapter Postgres (testcontainers ou compose), handler HTTP.

## Fora de escopo (v1)

- Persistir sessões ATIVAS (crash-recovery de mundo aberto) — mundos morrem com
  o game server de qualquer forma; reconcile já fecha como `abandoned`.
- Retenção/purga (volume dev é baixo; `closed_at` indexado deixa purga trivial).
- Timeline de eventos DENTRO da sessão (JOINED/DIED/BOSS_DEFEATED com timestamps)
  — os eventos já fluem via SNS pro analytics; se o produto pedir "linha do
  tempo da sala", vira spec própria em cima do envelope existente.
- Superfície pública (fora do admin) — API de "minhas partidas" pro jogador
  final é outra spec (auth de player, rate limit, shape público).
