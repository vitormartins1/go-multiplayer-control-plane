# Plan: 025-session-history

## Decisões técnicas

### D1 — Onde persistir: Postgres próprio do session-service
O session-service é o ÚNICO serviço stateful-por-natureza sem banco (tudo
in-memory). Alternativas avaliadas:

- **(a) Postgres do session-service** ✅ — dono do ciclo de vida grava o que
  fecha; padrão idêntico aos outros 7 serviços (migrations por volume, exporter,
  compose). Sem novo salto de rede no caminho quente (só no close).
- (b) Materializar no analytics-service a partir do SNS — rejeitado: o CLOSE
  não flui por SNS hoje (só participation events e resultados); analytics é
  rollup agregado, não fonte de sala individual; e criaria dependência de
  pipeline pra uma consulta operacional do admin.
- (c) Um "session-history-service" novo — rejeitado: cerimônia de um serviço
  inteiro pra duas consultas; YAGNI.

### D2 — Escrita síncrona best-effort no closeSession
`closeSession` já tem a regra "release do server primeiro, fan-out depois".
A gravação entra DEPOIS do release, síncrona (é 1 INSERT), com erro apenas
logado + métrica `session_history_persist_failures_total`. Sem fila/outbox na
v1 — se o Postgres estiver fora, perde-se o registro (aceitável em dev; a
métrica denuncia). Outbox/retry é evolução, não v1.

### D3 — Participants no domínio (papel por participante)
`RoleSlots.Occupants` no momento do close só contém os PRESENTES (DIED/LEFT
liberam o slot) — não serve pra reconstruir papel por participante. Logo:
`Session.Participants []Participant{PlayerID, Role, JoinedAt}` acumulado em
`OpenWorld` (host), `JoinWorld` (phantom, com dedupe — irmão do dedupe de
`Players` do fix 2026-07-12) e `Create` (match, papel vazio/`team:<id>`).
`Players []string` continua existindo como derivado (compat com DTOs atuais);
na v2 pode ser removido dos DTOs.

### D4 — Schema (2 tabelas, write-once)
```sql
CREATE TABLE closed_sessions (
  session_id   TEXT PRIMARY KEY,
  match_id     TEXT NOT NULL DEFAULT '',
  game_id      TEXT NOT NULL,
  mode_id      TEXT NOT NULL DEFAULT '',
  archetype    TEXT NOT NULL DEFAULT '',
  origin       TEXT NOT NULL,
  host_player_id TEXT NOT NULL DEFAULT '',
  close_reason TEXT NOT NULL,
  profile_hash TEXT NOT NULL DEFAULT '',
  opened_at    TIMESTAMPTZ NOT NULL,
  closed_at    TIMESTAMPTZ NOT NULL
);
CREATE INDEX idx_closed_sessions_game ON closed_sessions (game_id, closed_at DESC);

CREATE TABLE closed_session_players (
  session_id TEXT NOT NULL REFERENCES closed_sessions (session_id),
  player_id  TEXT NOT NULL,
  role       TEXT NOT NULL DEFAULT '',
  joined_at  TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (session_id, player_id)
);
CREATE INDEX idx_closed_players_lookup ON closed_session_players (player_id, session_id);
```
Consulta por player+jogo: join das duas (índice de player + filtro game_id na
principal). Cursor de paginação: `closed_at` (+ `session_id` como desempate).

### D5 — Todos os arquétipos desde a v1
`closeSession` é único; discriminar seria trabalho extra. Campos por-arquétipo
ficam nos defaults ('' / vazio). A aba History do player funciona de graça pros
4 arquétipos.

### D6 — Fallback de leitura
Postgres indisponível na leitura → 503 com `history_unavailable` no endpoint
`state=closed`/player history; a aba Sessions cai pro comportamento atual
(ring) com aviso. Sem cache além do ring.

## Riscos / Gotchas conhecidos

- **Migrations só rodam em volume novo** (gotcha operacional do repo): primeira
  subida exige recriar o volume do postgres novo — documentar no compose.
- Compose: novo `session-postgres` + exporter + wiring de envs (`SESSION_DB_URL`)
  — seguir o padrão do achievements-service (mais recente).
- `closed_session_players.role` de match fica vazio na v1 — a UI mostra "-".
- O ring atual mistura jogos; o endpoint novo é por jogo — atenção pra não
  regredir a aba Rooms global (ela continua no caminho atual).

## Ordem de implementação

Fatia 1 (session-service, core): D3 Participants + store Postgres + escrita no
close + testes. Fatia 2: endpoints admin (closed por jogo; por player). Fatia 3:
admin-bff (proxy + handles). Fatia 4: admin-ui (aba History no PlayerDetail +
paginação na aba Sessions). Fatias 3-4 têm spec-irmã curta nos respectivos
serviços quando forem implementadas (padrão das specs cross-service, ex. 020).
