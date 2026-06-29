# matchmaking-service — testando o gRPC

Diferente dos serviços REST (que têm `.http`), o matchmaking é **gRPC streaming**
(ADR-0003). Para testar manualmente, use [`grpcurl`](https://github.com/fullstorydev/grpcurl)
(o servidor tem **reflection** habilitada, então não precisa do `.proto`).

```bash
# Requer a stack no ar:
#   docker compose -f infra/docker/docker-compose.yml up

# 1. token do identity (REST)
TOKEN=$(curl -s -XPOST localhost:8081/sessions \
  -H 'Content-Type: application/json' \
  -d '{"handle":"Player_1","password":"supersecret"}' | jq -r .token)
# (crie a conta antes: POST /accounts com o mesmo handle/senha)

# 2. listar os serviços/métodos (via reflection)
grpcurl -plaintext localhost:50051 list
grpcurl -plaintext localhost:50051 list matchmaking.v1.Matchmaking

# 3. entrar na fila (server-streaming). Como o match exige matchSize=2 jogadores,
#    rode DOIS terminais com tokens de jogadores diferentes:
grpcurl -plaintext \
  -H "authorization: Bearer $TOKEN" \
  -d '{}' \
  localhost:50051 matchmaking.v1.Matchmaking/FindMatch
# -> { "status": "STATUS_SEARCHING" }
# -> { "status": "STATUS_MATCHED", "matchId": "...", "players": [...], "connectInfo": "mock://session/..." }

# sem o header authorization -> erro Unauthenticated
```

Acompanhe a fila no Grafana: a métrica `matchmaking_queue_size` (HTTP em :8086/metrics).
O `connectInfo` é mock até o session-service existir.
