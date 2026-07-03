# Spec: 005-admin-sessions

**Serviço:** session-service
**Status:** Draft
**Data:** 2026-07-03

## Contexto

Superfície **admin** do session-service para o painel (admin-bff `007-sessions-panel` → admin-ui `007-sessions-tab`): dentro da página de um jogo, uma aba **Sessions** lista as partidas — abertas e fechadas — para operação e debug (ver players da sala, saber se fechou e por quê).

Depende da spec `004-agones-allocator` (estado OPEN/CLOSED + motivo). Acrescenta: **histórico bounded** de sessões fechadas (memória) e o **endpoint HTTP `/admin`** no padrão dos outros serviços consumidos pelo BFF.

## Comportamento

### Histórico de fechadas

- WHEN uma sessão fechar (qualquer motivo) → ela SHALL ir para um **histórico em memória, bounded** (ring buffer, `SESSION_HISTORY_LIMIT` default 200, global), com todos os campos + `closed_at`/`close_reason`.
- O histórico é **best-effort de dev** (memória, some no restart) — persistência real é evolução, fora do escopo.

### Endpoint admin

- `GET /admin/games/{gameId}/sessions` SHALL responder:
  ```json
  {
    "active":  [ {"sessionId","matchId","players":["..."],"serverAddress","state":"open","openedAt"} ],
    "closed":  [ { ..., "state":"closed","closedAt","closeReason":"result|ttl" } ]
  }
  ```
  `active` = sessões abertas do jogo (ordem: mais recente primeiro); `closed` = histórico filtrado pelo jogo (mais recente primeiro).
- Query param opcional `state=open|closed` filtra a resposta.
- THE ENDPOINT SHALL seguir o mesmo padrão dos `/admin` dos outros serviços (mesmo mux HTTP das métricas/JWKS; proteção igual à dos pares — conferir no plan como leaderboard/identity fazem).

## Casos de Teste / Aceite

- [x] Sessão criada → aparece em `active` com players/server/openedAt
- [x] Fechou por `result` e por `ttl` → aparece em `closed` com motivo e `closedAt`
- [x] Histórico respeita o limite (mais antiga sai primeiro)
- [x] Filtro `state=` e filtro por `gameID` corretos (multi-tenant: sessão de outro jogo não vaza)
- [x] Restart → histórico vazio, `active` reconstruída do estado em memória (vazia também — documentado)
- [x] Testes httptest do handler

## Fora do escopo

- Persistência (Postgres/Redis) do histórico
- Paginação (limite bounded já segura o payload)
- Auth serviço-a-serviço real (ADR-0018)
- Visão da fleet Agones (contadores Ready/Allocated) — possível evolução da aba
