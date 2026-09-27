# Plan: 032-world-heartbeat

| Tema | Decisão | Por quê |
|---|---|---|
| Opt-in por mundo | `Session.LastHeartbeatAt` zero = regra antiga (idade) | compatível com jogo e DS que ainda não mandam heartbeat |
| Motivo de fechamento | `abandoned` (o mesmo da reconciliação de órfãs) | é o mesmo fato — o host/servidor sumiu sem avisar — e não cria vocabulário novo no analytics/admin |
| TTL | env `WORLD_HEARTBEAT_TTL`, padrão 90 s; intervalo sugerido = TTL/3 | tolera perder dois heartbeats seguidos |
| Onde a regra mora | `Service.Sweep` (já roda a cada minuto) | sem goroutine nova; a granularidade de 1 min basta |
| Permissões | player (host do mundo), game_server, world_host (só o próprio mundo) | mesma tabela da ADR-0030/spec 031 |

Arquivos: `proto/session/v1/session.proto` (+ `gen/`), `internal/domain/session.go`,
`internal/service/{session_service.go,heartbeat.go}`, `internal/transport/{auth.go,grpc_server.go}`,
`cmd/server/main.go`, docs de contrato.
