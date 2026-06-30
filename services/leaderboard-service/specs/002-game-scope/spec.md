# Spec: 002-game-scope

**Serviço:** leaderboard-service
**Status:** Approved
**Data:** 2026-06-30

## Contexto

Torna o ranking **por jogo** (multi-tenant, ADR-0017) e adiciona o que o fan-out e o painel precisam: um **submit server-side** (o session reporta o score do jogador, sem JWT — ADR-0018) e um **read admin top-N por jogo**. Cada jogo é um sorted set Redis próprio (`leaderboard:{gameID}`).

## Comportamento

### Player-facing (X-Game-Id)
- WHEN `POST /scores {score}` / `GET /leaderboard` / `GET /leaderboard/me` com header `X-Game-Id=g` -> opera no ranking do jogo `g` (highscore/top/around). IF `X-Game-Id` ausente -> `400`.

### Admin (internal key, ADR-0018; jogo/jogador no path)
- WHEN `POST /admin/games/{gameId}/players/{playerID}/scores {score}` -> aplica o highscore do jogador no ranking do jogo (server-authoritative; é o que o fan-out do session usa).
- WHEN `GET /admin/games/{gameId}/leaderboard?limit=` -> top-N do jogo (`[{rank, playerId, score}]`).

### Invariante
- THE SYSTEM SHALL escopar todo submit/leitura por `game_id` (sorted set por jogo); rankings de jogos diferentes não se misturam.

## Contratos de Interface

| Método | Rota | Auth |
|---|---|---|
| POST | `/scores` | JWT + `X-Game-Id` |
| GET | `/leaderboard` · `/leaderboard/me` | JWT + `X-Game-Id` |
| POST | `/admin/games/{gameId}/players/{playerID}/scores` | internal key |
| GET | `/admin/games/{gameId}/leaderboard?limit=` | internal key |

- `LeaderboardStore` e `LeaderboardService` ganham `gameID` em todos os métodos; novo `SubmitFor(ctx, gameID, playerID, score)` (admin) e `Top(ctx, gameID, limit)`.

## Casos de Teste Obrigatórios
- [ ] submit/top isolados por jogo (score no jogo A não aparece no B)
- [ ] player-facing sem `X-Game-Id` -> `400`
- [ ] `POST /admin/games/{g}/players/{p}/scores` aplica highscore (server-side)
- [ ] `GET /admin/games/{g}/leaderboard` -> top-N do jogo
- [ ] `/admin/...` exige internal key (guard no main)

## Fora do escopo
- **Fan-out do session -> leaderboard** — feature seguinte (session)
- **Consumo no admin-bff/admin-ui** (aba Leaderboards) — features seguintes
- Migração do ranking global antigo (Redis dev; recomeça por jogo)
- Janelas/seasons por período — backlog
