# Tasks: 003-game-scope
**Plan:** specs/003-game-scope/plan.md
**Data:** 2026-06-29

## matchmaking
- [x] proto `FindMatchRequest{game_id}` + `buf generate`
- [x] domain: `Join(ctx, gameID, playerID)`; `Provision(ctx, matchID, players, gameID)`
- [x] service: `ticket.gameID`; `tryForm` agrupa por jogo; `provision` passa gameID
- [x] service test: isolamento por jogo; gameID propagado
- [x] transport: `FindMatch` game_id (vazio -> InvalidArgument) + test
- [x] sessionclient: `Provision` -> `CreateSession{game_id}` + test

## session (companion)
- [x] proto `CreateSessionRequest{game_id}` + `buf generate`
- [x] domain: `Session.GameID`; `Create(ctx, matchID, players, gameID)`
- [x] service: `Create` guarda gameID + test
- [x] transport: `CreateSession` game_id + test

## verificação
- [x] `go test ./... -race` + `go build ./...`
- [x] Docker: mesmo jogo -> match; jogos diferentes -> não casa; sessão guarda game_id
