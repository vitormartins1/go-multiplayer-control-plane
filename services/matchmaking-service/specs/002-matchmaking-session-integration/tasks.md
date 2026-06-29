# Tasks: 002-matchmaking-session-integration
**Plan:** specs/002-matchmaking-session-integration/plan.md
**Data:** 2026-06-29

> Ordem: proto/codegen → domain → sessionclient(adapter) → service → transport → wire-up/infra.
> Matchmaking vira cliente gRPC do session. Testes acompanham cada camada (service/adapter com -race).
> Marcar [x] conforme implementa.

## Fase 1 — Proto e codegen (buf)
- [ ] `proto/matchmaking/v1/matchmaking.proto` — `TicketUpdate`: remover `connect_info`; adicionar `server_address`, `connect_token`, `reason`; `Status`: adicionar `STATUS_FAILED`
- [ ] `buf generate` → regenerar `gen/matchmaking/v1/*.pb.go` (commitado); `go build` do gerado

## Fase 2 — Domain
- [ ] `internal/domain/matchmaking.go` — Port `SessionProvisioner` (`Provision(ctx, matchID, players) (Provisioned, error)`); tipos `Provisioned{ServerAddress, Tokens map[string]string}` e `MatchResult{MatchID, Players, ServerAddress, ConnectToken, Failed, Reason}`; mudar `Matchmaker.Join` para `<-chan MatchResult`; remover `Match.ConnectInfo`
- [ ] `internal/domain/errors.go` — `ErrNoServer` (mapeado de `ResourceExhausted`)

## Fase 3 — Sessionclient (adapter de saída)
- [ ] `internal/sessionclient/grpc_client.go` — implementa `domain.SessionProvisioner` sobre `sessionv1.SessionClient`; mapeia `grants[]`→`Provisioned{ServerAddress, Tokens}`; `codes.ResourceExhausted`→`domain.ErrNoServer`
- [ ] `internal/sessionclient/grpc_client_test.go` — bufconn com o **session real in-process** (allocator + token + publisher): provisiona ok (tokens por jogador) e pool esgotado → `ErrNoServer`

## Fase 4 — Service
- [ ] `internal/service/matchmaker.go` — `New` recebe `SessionProvisioner`; `ticket.ch` vira `chan domain.MatchResult` (buffer 1); `tryForm` agrupa sob lock e dispara `go provision(ctx, matchID, group)`; `provision` chama o provisioner, entrega token por jogador no sucesso e `MatchResult{Failed, Reason}` na falha, fecha os canais; propagar `ctx` do `Run`; timeout na chamada; contadores `matches_formed`/`matches_failed`
- [ ] `internal/service/matchmaker_test.go` — mock de `SessionProvisioner`; atualizar testes da 001 (sem `ConnectInfo`); cobrir: sucesso entrega token do próprio jogador; `ErrNoServer`→FAILED; erro genérico→FAILED; re-join após falha; rodar `-race`

## Fase 5 — Transport (gRPC)
- [ ] `internal/transport/grpc_server.go` — `FindMatch` consome `MatchResult`: `Failed`→`TicketUpdate{STATUS_FAILED, reason}`; sucesso→`TicketUpdate{STATUS_MATCHED, matchId, players, server_address, connect_token}`
- [ ] `internal/transport/grpc_server_test.go` — fake `SessionProvisioner`; MATCHED traz `server_address` + `connect_token` por jogador; `ErrNoServer`→`STATUS_FAILED`

## Fase 6 — Wire-up, infra e verificação
- [ ] `cmd/server/main.go` — ler `SESSION_GRPC_ADDR`; `grpc.NewClient` para o session (insecure + otelgrpc client handler p/ trace cross-service); criar o `sessionclient`; injetar no `service.New`; fechar a conn no shutdown
- [ ] `services/matchmaking-service/api/README.md` — atualizar o exemplo MATCHED (`server_address` + `connect_token`) e documentar `STATUS_FAILED`
- [ ] `infra/docker/docker-compose.yml` — `matchmaking-service`: env `SESSION_GRPC_ADDR: session-service:50051` + `depends_on: session-service`
- [ ] `go test ./... -race` verde; `go build ./...`
- [ ] Validar no Docker: 2 clientes no FindMatch → MATCHED com `server_address` + `connect_token` verificável no JWKS do session; saturar o pool do session → `STATUS_FAILED`; trace FindMatch→CreateSession no Tempo
