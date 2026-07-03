# Plan: 007-sessions-panel

**Spec:** [spec.md](spec.md)
**Data:** 2026-07-03

## 1. Client (`internal/session/client.go`)

- Padrão do `internal/leaderboard`: `New(baseURL, httpClient)`, `SessionsByGame(ctx, gameID, state)` → structs internas espelhando o JSON do serviço. Testes com httptest fake.

## 2. Enriquecimento de handles

- No handler (transport), coletar os playerIDs únicos (active+closed) e resolver via client identity existente (conferir se há lookup por id em lote; se for 1-a-1, limitar concorrência simples). Falha → handle `""`.

## 3. Transport

- `GET /api/games/{gameId}/sessions` no mux; DTO: `{active:[], closed:[]}`, cada item `{sessionId, matchId, state, players:[{id, handle}], serverAddress, openedAt, closedAt?, closeReason?}`.
- Erros: 502 `sessions_unavailable`; padrão `writeError` existente.

## 4. Wiring / compose / topologia

- `SESSION_URL` no main (obrigatória como as demais? conferir se os pares usam fatal ou default) + compose.
- Aresta session no `/api/topology`.

## Riscos / notas

- Identity: se não houver endpoint de lookup por player id, o enriquecimento vira “SHOULD adiado” — registrar no fechamento e mostrar ids truncados no UI (decisão já prevista lá).
- Ordem: session-service 005 mergeada antes (contrato do JSON).
