# Plan: 003-game-scope
**Spec:** specs/003-game-scope/spec.md
**Data:** 2026-06-29

## Abordagem técnica
`game_id` entra no proto dos dois gRPC e vira parâmetro nas Ports. O matchmaker
agrupa por `game_id` (mantém a fila única; `tryForm` forma matchSize tickets do
**mesmo jogo**). O session guarda `game_id` na `Session`.

## Arquivos a modificar
**matchmaking**
- `proto/matchmaking/v1/matchmaking.proto` — `FindMatchRequest{ game_id }`; `buf generate`
- `internal/domain/matchmaking.go` — `Matchmaker.Join(ctx, gameID, playerID)`; `SessionProvisioner.Provision(ctx, matchID, players, gameID)`
- `internal/service/matchmaker.go` — `ticket.gameID`; `Join` recebe gameID; `tryForm` agrupa por jogo (matchSize do mesmo `game_id`); `provision` passa `gameID`
- `internal/service/matchmaker_test.go` — Join com gameID; teste de isolamento por jogo; provisioner fake recebe gameID
- `internal/transport/grpc_server.go` — `FindMatch` lê `req.GetGameId()` (vazio -> `InvalidArgument`); passa ao `Join`
- `internal/transport/grpc_server_test.go` — game_id no request; fake provisioner com gameID
- `internal/sessionclient/grpc_client.go` — `Provision(ctx, matchID, players, gameID)` -> `CreateSession{game_id}`
- `internal/sessionclient/grpc_client_test.go` — fake session server confere o game_id
- `cmd/server/main.go` — sem mudança (assinaturas internas)

**session (companion)**
- `proto/session/v1/session.proto` — `CreateSessionRequest{ game_id }`; `buf generate`
- `internal/domain/session.go` — `Session.GameID`; `SessionService.Create(ctx, matchID, players, gameID)`
- `internal/service/session_service.go` — `Create` guarda `gameID`
- `internal/service/session_service_test.go` — Create com gameID; asserir guardado
- `internal/transport/grpc_server.go` — `CreateSession` lê `req.GetGameId()`; passa ao `Create`
- `internal/transport/grpc_server_test.go` — game_id no request

## Decisões
- **Fila única, agrupamento por jogo no `tryForm`** (vs filas por jogo): menos refactor; `tryForm` parte os tickets por `game_id` e forma matchSize do mesmo jogo.
- **`game_id` obrigatório no FindMatch** (InvalidArgument se vazio): o cliente sempre sabe seu jogo.
- **session valida pouco**: confia no `game_id` do matchmaking (control plane interno).

## Ordem
1. matchmaking proto + buf; domain; service (+isolamento) + tests; transport + tests; sessionclient + tests
2. session proto + buf; domain; service; transport; tests
3. `go test ./... -race` + `go build`; validar no Docker (2 clientes mesmo jogo -> match; jogos diferentes -> não casa; sessão guarda game_id)

## Riscos
- **buf** regenera os dois protos (matchmaking e session).
- **Churn de testes** nos dois serviços (assinaturas Join/Provision/Create).
- **`tryForm` por jogo**: cobrir o isolamento (A não casa com B) com teste.
