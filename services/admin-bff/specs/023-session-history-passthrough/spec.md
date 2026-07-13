# Spec: 023-session-history-passthrough

**Serviço:** admin-bff
**Status:** Done
**Data:** 2026-07-13
**Parte de:** session-service/025-session-history (Fatia 3).

## Contexto

O session-service (spec 025) ganhou histórico persistido de sessões fechadas,
lido via dois endpoints admin novos, paginados por `closed_at`:
`GET /admin/games/{gameId}/sessions?state=closed&limit=&before=` (mudou de fonte
— ring in-memory → store Postgres) e `GET /admin/players/{playerId}/sessions?
gameId=&limit=&before=` (novo). O BFF repassa os dois e enriquece os
participantes com o handle do identity (mesmo padrão de `GameSessions`/`Rooms`).

## Comportamento

- `GET /api/games/{gameId}/sessions?state=closed&limit=&before=` — quando
  `state=closed`, o BFF desvia para o store persistido (em vez do ring usado
  pelos demais valores de `state`) e devolve `{closed: [...], nextBefore?}` em
  vez de `{active, closed}`. `limit`/`before` repassados 1:1; `limit` inválido
  (não-numérico ou <= 0) → 400 `invalid_pagination`.
- `GET /api/players/{playerId}/sessions?gameId=&limit=&before=` (novo) — lista
  as sessões fechadas do jogo em que o player participou. `gameId` obrigatório
  → 400 `missing_game_id` se ausente.
- Cada sessão devolvida carrega `participants: [{playerId, handle, role,
  joinedAt}]` — a base do histórico (papel por participante); `players` segue
  existindo por compat.
- Handle resolvido no identity, best-effort (mesmo padrão de
  `GameSessions`/`Rooms`): identity fora → handle vazio, sem derrubar a
  resposta.
- IF o Postgres do histórico no session-service estiver fora (503
  `history_unavailable` upstream) → o BFF repassa 503 `history_unavailable`
  (distinto do 502 `sessions_unavailable` de session-service totalmente
  inacessível).

## Casos de Teste / Verificação

- [x] `state=closed` lê do store (não mais do ring), paginado, participantes
      com handle resolvido
- [x] `limit`/`before` repassados; `limit` inválido → 400
- [x] Postgres do histórico fora → 503 `history_unavailable` (distinto de
      `sessions_unavailable`)
- [x] `GET /api/players/{playerId}/sessions` sem `gameId` → 400
      `missing_game_id`; com `gameId` → lista paginada com handles
