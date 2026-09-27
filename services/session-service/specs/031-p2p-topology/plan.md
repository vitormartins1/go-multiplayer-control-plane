# Plan: 031-p2p-topology

## Decisões técnicas

| Tema | Decisão | Por quê |
|---|---|---|
| Credencial do host p2p | `world_token` opaco (32 bytes aleatórios, base64url), emitido no `OpenWorld`; o session guarda só o SHA-256 e um índice hash → mundo | o JWT do identity dura 15 min e um mundo dura horas; o token vive e morre com o mundo e só vale para ele |
| Onde o token é aceito | metadata `x-world-token`; o interceptor da ADR-0030 resolve o mundo e marca o chamador como `world_host` | reaproveita a tabela de permissões por RPC; a checagem "é deste mundo" fica no `hostCheck` que já existe |
| RPCs do `world_host` | `ReportParticipationEvent`, `CloseWorld`, `LeaveWorld`, `GetSessionConfig` — sempre só do próprio mundo | é o que o Dedicated Server faz hoje; `ReportResult` é de `queue_arena`, fica de fora |
| Claim `server` do connect token | em p2p recebe o `host_connection` | o host confere que o token foi emitido para ele, como o DS confere o próprio endereço |
| Sem GameServer | `closeSession` só chama `Release` quando há `ServerID`; o `reconcile` ignora mundos sem `ServerID` | senão o reconcile fecharia todo mundo p2p como órfão no primeiro tick |
| Retorno do `JoinWorld` (service) | `domain.WorldEndpoint{ServerAddress, HostConnection}` no lugar da string | um só valor para o transport e o matchmaking repassarem |
| Registry | `p2p` válido só em `host_world_dropin` (`ErrUnsupportedTopology` nos outros) | spec, item 7 |
| Admin | JSON da sessão ganha `topology`; Salas/Sessões mostram "P2P · host"; o seletor de topologia libera p2p só no `host_world_dropin` | spec, item 8 |

## Arquivos

- `services/session-service/proto/session/v1/session.proto` (+ `gen/`), `services/matchmaking-service/proto/matchmaking/v1/matchmaking.proto` (+ `gen/`)
- session: `internal/domain/{session.go,errors.go}`, `internal/service/session_service.go`,
  `internal/transport/{auth.go,grpc_server.go,http_handler.go}`, `cmd/server/main.go`
- matchmaking: `internal/domain/{invasion.go,coop.go}`, `internal/sessionclient/grpc_client.go`,
  `internal/service/{invader.go,summoner.go}`, `internal/transport/grpc_server.go`
- game-registry: `internal/service/profile_service.go`, `internal/domain/game.go`
- admin-bff: `internal/session/client.go`, `internal/domain/sessions.go`
- admin-ui: `src/lib/api.ts`, `src/pages/{MultiplayerSettings,Rooms,GameSessions}.tsx`
- docs: `docs/contracts/CHANGELOG.md` (1.4.0), contrato do `host_world_dropin`, ADR-0032, `docs/GO-LEARNINGS.md`

## Limitação conhecida

O histórico persistido (`closed_sessions`, spec 025) não guarda a topologia: sessões fechadas lidas do Postgres
chegam sem `topology` e o admin mostra "—" na coluna de servidor. As em memória (abertas e recém-fechadas) mostram
"P2P · host". Adicionar a coluna é uma migration pequena quando fizer falta.
