# Spec: 007-sessions-panel

**Serviço:** admin-bff
**Status:** Draft
**Data:** 2026-07-03

## Contexto

Expor as sessões de um jogo (session-service `005-admin-sessions`) para a aba **Sessions** do admin-ui (`007-sessions-tab`): partidas abertas/fechadas, players, server, timestamps. Padrão dos outros painéis: client HTTP interno + rota `/api` fina.

## Comportamento

- `GET /api/games/{gameId}/sessions` (querystring `state=` repassada) → proxy para `GET {SESSION_URL}/admin/games/{gameId}/sessions`, resposta re-serializada no DTO do BFF (camelCase, como pares).
- THE BFF SHOULD **enriquecer os players com handle** (best-effort): resolver `playerId → handle` no identity-service (client já existente); falha no identity não derruba a resposta (handle vazio, id sempre presente).
- IF o session-service estiver fora → `502 sessions_unavailable` (padrão dos pares).
- Env nova: `SESSION_URL` (ex.: `http://session-service:8080`) — compose atualizado.
- Topologia (`/api/topology`): adicionar aresta admin-bff → session-service.

## Casos de Teste / Aceite

- [x] Proxy feliz: active/closed com campos completos e handles resolvidos
- [x] Identity fora → resposta ok com handles vazios (best-effort de verdade)
- [x] Session fora → 502 `sessions_unavailable`
- [x] `state=` repassado; httptest cobrindo tudo (fakes dos dois upstreams)

## Fora do escopo

- Cache/paginação; websocket/push (UI usa polling)
- Ações de admin sobre sessão (fechar na marra etc.) — evolução
