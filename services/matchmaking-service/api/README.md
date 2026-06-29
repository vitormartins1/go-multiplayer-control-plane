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
# -> { "status": "STATUS_MATCHED", "matchId": "...", "players": [...],
#      "serverAddress": "10.0.0.1:7777", "connectToken": "<JWT EdDSA do jogador>" }

# sem o header authorization -> erro Unauthenticated
```

Ao formar o match, o matchmaking chama o `CreateSession` do session-service
(ADR-0010): aloca um game server e emite **um connect token por jogador**. Cada
jogador recebe no seu stream o `serverAddress` e o **seu** `connectToken` (JWT
EdDSA verificável no JWKS do session em `:8087/.well-known/jwks.json`).

Se o session não tiver server livre (ou estiver indisponível), o update vem como
`{ "status": "STATUS_FAILED", "reason": "..." }` e o stream encerra — o cliente
chama `FindMatch` de novo.

Acompanhe no Grafana: `matchmaking_queue_size`, `matchmaking_matches_total` e
`matchmaking_matches_failed_total` (HTTP em :8086/metrics). O trace do FindMatch
segue até o CreateSession no Tempo (ADR-0014).
