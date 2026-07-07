# Spec: 013-board-passthrough

**Serviço:** admin-bff
**Status:** Draft
**Data:** 2026-07-06
**Parte de:** B2b do roadmap `queue_arena` (ADR-0023). Par: `admin-ui/017-board-selector`.

## Contexto

O leaderboard-service já serve múltiplos boards por jogo via `?board` (`score` + um por stat — fatia leaderboard/003). O admin-bff repassava só o board default (`score`): as chamadas admin não carregavam o param. Esta fatia **threada o `board`** pela cadeia (transport → service → adapter) na leitura (top-N) e no override de score (o endpoint admin do leaderboard-service já aceita `?board` nos dois).

## Comportamento

- WHEN `GET /api/games/{gameId}/leaderboard?board=<b>&limit=<n>` → THE SYSTEM SHALL repassar `board` ao leaderboard-service. `board` ausente/`""` → o leaderboard-service assume `score`.
- WHEN `PUT /api/games/{gameId}/leaderboard/{playerID}/score?board=<b>` → THE SYSTEM SHALL aplicar o override no board `<b>` (ausente → `score`).

## Contratos de Interface

- **Domain (`LeaderboardReader`):** `TopForGame(ctx, gameID, board string, limit int)` e `SetScore(ctx, gameID, playerID, board string, score int64)` — `board` novo em ambos.
- **Adapter `leaderboard/client.go`:** query param `board` (via `url.Values` no top-N; concat no scores); omitido quando vazio.
- **Service/Transport:** `GameLeaderboard`/`SetLeaderboardScore` +`board`; os handlers leem `r.URL.Query().Get("board")`.

## Casos de Teste Obrigatórios

- [ ] adapter: `TopForGame` monta `board=kills&limit=10`; board vazio → sem query; `SetScore` monta `?board=kills`
- [ ] transport: `GET .../leaderboard?board=kills` chega no service com `board="kills"`

## Fora do escopo

- Definir/validar boards (é do leaderboard-service); boards acumulativos (A2).
- Descobrir os boards disponíveis (o admin-ui os deriva do perfil — B2b/017).
