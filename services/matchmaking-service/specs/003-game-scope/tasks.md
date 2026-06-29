# Tasks: 003-game-scope
**Plan:** specs/003-game-scope/plan.md
**Data:** 2026-06-29

## matchmaking
- [ ] proto `FindMatchRequest{game_id}` + `buf generate`
- [ ] domain: `Join(ctx, gameID, playerID)`; `Provision(ctx, matchID, players, gameID)`
- [ ] service: `ticket.gameID`; `tryForm` agrupa por jogo; `provision` passa gameID
- [ ] service test: isolamento por jogo; gameID propagado
- [ ] transport: `FindMatch` game_id (vazio -> InvalidArgument) + test
- [ ] sessionclient: `Provision` -> `CreateSession{game_id}` + test

## session (companion)
- [ ] proto `CreateSessionRequest{game_id}` + `buf generate`
- [ ] domain: `Session.GameID`; `Create(ctx, matchID, players, gameID)`
- [ ] service: `Create` guarda gameID + test
- [ ] transport: `CreateSession` game_id + test

## verificação
- [ ] `go test ./... -race` + `go build ./...`
- [ ] Docker: mesmo jogo -> match; jogos diferentes -> não casa; sessão guarda game_id
