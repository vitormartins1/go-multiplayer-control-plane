# Tasks: 002-game-presence
**Plan:** specs/002-game-presence/plan.md
**ADR:** 0019
**Data:** 2026-06-30

## game-registry
- [x] migration 003 (game_players: PK, FK, índice)
- [x] domain presence + Ports (service/repo)
- [x] repository (upsert idempotente; PlayersOf; FK→ErrGameNotFound) + teste integração
- [x] service + transport (PUT/GET players) + teste transport
- [x] compose (migration no volume; recriado com down/rm -v)

## identity
- [x] domain PresenceNotifier; Authenticate/LoginAnonymous com gameID
- [x] adapter internal/presence (HTTP best-effort)
- [x] service (notifier nil-safe + log) + transport (gameId no body)
- [x] main (GAME_REGISTRY_URL + InternalKeyTransport) + compose; ajustar testes

## admin-bff
- [x] domain GamePlayer + GameRegistry.PlayersOf; adapter; service; transport (rota + 502) + testes

## admin-ui
- [x] api.gamePlayers; Players jogo-cêntrico (useGames)

## verificação
- [x] go test ./... -race + build/vet; npm run build
- [x] Docker E2E: login com gameId → base do jogo no painel (isolamento + idempotência + sem-gameId)
