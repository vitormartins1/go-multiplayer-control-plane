# Spec: 002-game-presence

**Serviço:** game-registry (núcleo) + identity-service, admin-bff, admin-ui (integração)
**Status:** Implementada (2026-06-30)
**Data:** 2026-06-30
**ADR:** 0019 (presença por jogo sobre identidade global)

## Contexto

Dá ao painel a **player base por jogo** sem quebrar a identidade global (ADR-0019). O game-registry passa a guardar **presença** — quem "está" em cada jogo — marcada no **login "dentro do jogo"**. Fecha a navegação jogo-cêntrica (jogo ativo → players do jogo → dados), iniciada em admin-ui/005.

## Comportamento

### game-registry (control plane interno, atrás da internal key — ADR-0018)
- WHEN `PUT /games/{gameId}/players/{playerID}` com `{ "handle": "..." }` → **upsert** de presença `(game_id, player_id, handle, first_seen)`. Idempotente: re-marcar não muda `first_seen`; atualiza `handle`. IF o jogo não existe → `404`.
- WHEN `GET /games/{gameId}/players` → lista a base do jogo: `[{ playerId, handle, firstSeen }]` por `first_seen` (asc).

### identity-service (login-into-game)
- `POST /sessions` e `POST /sessions/anonymous` aceitam `gameId` **opcional** no corpo.
- WHEN o login tem sucesso E `gameId` presente → marca presença no game-registry (`PUT .../players/{sub}` com o handle). **Best-effort:** falha loga e **não** derruba o login (devolve o token normalmente). IF `gameId` ausente → comportamento atual (só token).

### admin-bff
- WHEN `GET /api/games/{gameId}/players` → repassa a base do jogo (game-registry). Indisponibilidade → `502` (`game_registry_unavailable`).

### admin-ui (Players jogo-cêntrico)
- A aba **Players** lista a base do **jogo ativo** (header). Sem jogo ativo → orienta a escolher. Clicar num player abre o detalhe (que já usa o jogo ativo).

## Contratos

- game-registry: `Presence{PlayerID, Handle, FirstSeen}`; Ports `MarkPresence(ctx, gameID, playerID, handle)`, `PlayersOf(ctx, gameID) []Presence`. Migration `003_create_game_players.sql`.
- identity: domain Port `PresenceNotifier.MarkPresence(ctx, gameID, playerID, handle)`; adapter HTTP `internal/presence` (injeta internal key); `Authenticate`/`LoginAnonymous` ganham `gameID`. Config `GAME_REGISTRY_URL`.
- admin-bff: `GamePlayer{PlayerID, Handle, FirstSeen}`; estende `GameRegistry` com `PlayersOf(ctx, gameID)`; rota nova.
- admin-ui: `api.gamePlayers(gameId)`; `Players` usa o contexto de jogo.

## Casos de Teste Obrigatórios
- [ ] (game-registry) upsert idempotente (first_seen estável); jogo inexistente → 404; PlayersOf lista por first_seen — testcontainers
- [ ] (game-registry) `/games/...` exige internal key
- [ ] (identity) login com gameId marca presença (best-effort: falha do registry não derruba o login)
- [ ] (identity) login sem gameId não chama o registry
- [ ] (admin-bff) `/api/games/{g}/players` 200 + 502 quando fora
- [ ] (admin-ui) Players lista a base do jogo ativo; sem jogo → orienta; build ok

## Fora do escopo
- Reconciliação do `handle` desnormalizado quando o usuário troca de handle — evolução
- Remover presença / "sair do jogo" — incremental
- `coverUrl` do jogo — feature à parte
