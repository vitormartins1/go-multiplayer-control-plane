# Plan: 002-game-presence
**Spec:** specs/002-game-presence/spec.md
**ADR:** 0019
**Data:** 2026-06-30

## game-registry (núcleo)
- `migrations/003_create_game_players.sql` — `game_players (game_id TEXT, player_id TEXT, handle TEXT, first_seen TIMESTAMPTZ, PK (game_id, player_id), FK game_id → games(id))`; índice por (game_id, first_seen).
- `internal/domain/presence.go` — `Presence{PlayerID, Handle, FirstSeen}`; Ports no service/repo: `MarkPresence(ctx, gameID, playerID, handle)`, `PlayersOf(ctx, gameID)`.
- `internal/repository/postgres_presence.go` — upsert `ON CONFLICT (game_id, player_id) DO UPDATE SET handle=EXCLUDED.handle` (first_seen estável); `PlayersOf` `ORDER BY first_seen`. FK viola (23503) → ErrGameNotFound.
- `internal/service` — valida jogo existe (ou confia na FK) + delega.
- `internal/transport` — `PUT /games/{gameId}/players/{playerID}` (body handle), `GET /games/{gameId}/players`.
- testes: repo (testcontainers, idempotência/404), transport (unit).

## identity (login-into-game)
- `internal/domain` — Port `PresenceNotifier`; `Authenticate`/`LoginAnonymous` ganham `gameID string`.
- `internal/presence/client.go` — adapter HTTP (PUT no game-registry; best-effort, erro só logado no caller).
- `internal/service/auth_service.go` — `service` ganha `notifier` (nil-safe) + `log`; marca presença best-effort após sucesso quando gameID != "".
- `internal/transport` — `credentials` ganha `GameId`; repassa aos métodos.
- `cmd/server/main.go` — `GAME_REGISTRY_URL`; httpClient com `auth.InternalKeyTransport`; wire do notifier.
- testes: ajustar assinaturas; login com/sem gameId.

## admin-bff
- `internal/domain/admin.go` — `GamePlayer{PlayerID, Handle, FirstSeen}`; estende `GameRegistry` com `PlayersOf(ctx, gameID)`.
- `internal/gameregistry/client.go` — `PlayersOf` (GET /games/{id}/players).
- `internal/service` + `internal/transport` — método + rota `GET /api/games/{gameId}/players` (502 via writeUpstream).
- testes: adapter httptest, service, transport.

## admin-ui
- `lib/api.ts` — `GamePlayer` + `api.gamePlayers(gameId)`.
- `pages/Players.tsx` — usa `useGames()` (jogo ativo); lista a base do jogo; sem ativo → orienta. Mantém clicar → /players/:id.

## Ordem
1. game-registry (migration→domain→repo→service→transport) + testes + compose (migration volume + down -v)
2. identity (notifier+adapter+login) + main/compose + testes
3. admin-bff (domain→adapter→service→transport) + testes
4. admin-ui (api→Players)
5. build/test/vet; Docker E2E (login-into-game popula a base; painel lista)

## Riscos
- **Migration em volume existente** do game-registry-postgres → `down -v` (perde os jogos de teste).
- **Assinatura de Authenticate/LoginAnonymous** rippla nos testes do identity (igual aos game-scope anteriores).
- Presença best-effort: base só popula após login-into-game (documentado).
