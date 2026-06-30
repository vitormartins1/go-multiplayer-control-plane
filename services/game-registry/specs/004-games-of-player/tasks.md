# Tasks: 004-games-of-player
**Spec:** specs/004-games-of-player/spec.md
**Data:** 2026-06-30

## game-registry (consulta reversa)
- [x] domain PresenceService.GamesOf + PresenceRepository.GamesByPlayer
- [x] repo GamesByPlayer (JOIN game_players × games, por nome) + teste integração
- [x] service GamesOf; transport GET /players/{playerID}/games + teste
- [x] guard cobre /players além de /games

## admin-bff
- [x] GameRegistry.GamesOfPlayer; adapter; service PlayerGames; GET /api/players/{id}/games (502) + testes

## admin-ui (navegação player-cêntrica — remove o jogo ativo global)
- [x] api.gamesOfPlayer
- [x] remove GameProvider/useGames + GameSwitcher do header (fonte do ruído)
- [x] Games: useAsync(api.games); clique navega ao detalhe
- [x] GameDetail: nova aba **Players** (base do jogo, presença) — GamePlayers
- [x] Players: base GLOBAL (identity, paginada) + avatares; sem exigir jogo
- [x] PlayerDetail: seletor escopado aos jogos do player (default 1º); sem jogos → avisa

## verificação
- [x] go test ./... -race + build/vet; npm run build
- [x] Docker E2E: reverse presence (Nova→2 jogos, Bjorn→1); base global=20; guard /players 401
