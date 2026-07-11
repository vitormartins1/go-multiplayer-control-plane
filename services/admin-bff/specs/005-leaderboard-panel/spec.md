# Spec: 005-leaderboard-panel

**Serviço:** admin-bff (+ admin-ui)
**Status:** Done
**Data:** 2026-06-30

## Contexto

Acende a aba **Leaderboards** do detalhe do jogo no painel (hoje "em breve"). Fatia vertical fina: o admin-bff lê o top-N do leaderboard (que ganhou o `GET /admin/games/{gameId}/leaderboard` na 002-game-scope), e o admin-ui o exibe numa tabela. O BFF já injeta a internal key no transport (ADR-0018).

## Comportamento

- WHEN `GET /api/games/{gameId}/leaderboard?limit=` (admin-bff) -> devolve o top-N do jogo (`[{rank, playerId, score}]`), lendo o leaderboard-service.
- IF o leaderboard estiver indisponível -> `502` (`leaderboard_unavailable`), como os outros upstreams.
- THE admin-ui SHALL trocar o `ComingSoon` da aba Leaderboards por uma tabela (rank, jogador, score) com carregamento on-open + botão Atualizar (sem auto-refresh, padrão do redesign).

## Contratos de Interface

- admin-bff: `GET /api/games/{gameId}/leaderboard?limit=` -> `{ "entries": [{rank, playerId, score}] }`
- Port `domain.LeaderboardReader.TopForGame(ctx, gameID, limit)`; adapter `internal/leaderboard` (HTTP) → `GET /admin/games/{g}/leaderboard`
- admin-ui: `api.gameLeaderboard(gameId, limit)`; componente `GameLeaderboards` na aba

## Casos de Teste Obrigatórios

- [ ] (bff) `GET /api/games/{g}/leaderboard` -> top-N do jogo (200)
- [ ] (bff) leaderboard fora -> `502`
- [ ] (ui) aba Leaderboards lista entries; estado vazio e erro tratados; build ok

## Fora do escopo

- Submeter score pelo painel (é server-authoritative, via fan-out)
- Paginação/seasons do ranking — top-N simples
- player-data/cloud-save tabs — seguem "em breve"
