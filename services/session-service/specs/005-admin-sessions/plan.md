# Plan: 005-admin-sessions

**Spec:** [spec.md](spec.md)
**Data:** 2026-07-03

Depende da 004 (estado/fechamento). Pequena: histórico + endpoint.

## 1. Histórico (service)

- `service/`: no `closeSession`, empurrar a sessão fechada para um ring buffer (slice circular, mutex — mesmo lock do mapa de sessões; `SESSION_HISTORY_LIMIT` default 200). Sem persistência.
- Query interna: `SessionsByGame(gameID) (active, closed []Session)` — active do mapa, closed do ring (filtro + ordem desc por timestamp).

## 2. Transport HTTP

- `transport/http`: `GET /admin/games/{gameId}/sessions` no mux existente (JWKS/metrics moram lá) — padrão do leaderboard-service: `/admin` **sem auth no serviço** (internal key na borda, ADR-0018).
- DTO JSON camelCase (como pares); `state=` filtra; 200 com listas vazias para jogo sem sessão (não 404 — jogo pode existir sem partidas).
- httptest: aberto/fechado/filtros/limite.

## 3. Config

- `SESSION_HISTORY_LIMIT` (default 200) no main.

## Riscos / notas

- Ordem dos merges: 004 primeiro (estado); esta spec não deve inventar campo próprio de estado.
- O ring é global e o filtro é por jogo — num histórico pequeno, um jogo barulhento pode "expulsar" o histórico de outro. Aceitável em dev; nota na spec do UI (empty state explica “memória, best-effort”).
