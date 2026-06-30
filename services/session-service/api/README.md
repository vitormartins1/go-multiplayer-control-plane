# session-service — testando o gRPC

O session é **gRPC unary** (ADR-0003), control plane interno (≈ GameLift). Para
testar manualmente use [`grpcurl`](https://github.com/fullstorydev/grpcurl) (o
servidor tem **reflection**, então não precisa do `.proto`). Diferente do
matchmaking, o MVP **não** exige JWT de jogador (o cliente fala com o matchmaking,
não com o session — ver spec).

```bash
# Requer a stack no ar:
#   docker compose -f infra/docker/docker-compose.yml up

# 1. listar serviços/métodos (via reflection)
grpcurl -plaintext localhost:50052 list
grpcurl -plaintext localhost:50052 list session.v1.Session

# 2. criar uma sessão a partir de um match -> aloca server + connect token por jogador
grpcurl -plaintext \
  -d '{"matchId":"match-1","players":["p1","p2"],"gameId":"00000000-0000-0000-0000-000000000000"}' \
  localhost:50052 session.v1.Session/CreateSession
# -> { "sessionId": "...", "serverAddress": "10.0.0.1:7777",
#      "grants": [ {"playerId":"p1","connectToken":"<JWT EdDSA>"}, ... ] }

# 3. validar o connect token offline (como faz o Unity Dedicated Server):
#    a chave pública vem do JWKS exposto pelo próprio session.
curl -s localhost:8087/.well-known/jwks.json | jq
# cole o connectToken em https://jwt.io ou decodifique os claims:
#   sub = playerId, match_id, server (endereço), exp

# 4. reportar o resultado -> fan-out (stub que loga), libera o server, encerra a sessão
grpcurl -plaintext \
  -d '{"matchId":"match-1","results":[{"playerId":"p1","score":10,"won":true}]}' \
  localhost:50052 session.v1.Session/ReportResult
# -> {}

# match inexistente -> NotFound
grpcurl -plaintext -d '{"matchId":"nope"}' localhost:50052 session.v1.Session/ReportResult

# pool esgotado (3 servers no pool mock) -> ResourceExhausted no 4º CreateSession
```

Acompanhe no Grafana a métrica `session_active_sessions` (HTTP em :8087/metrics).
O fan-out do resultado é um **stub que loga** no MVP — o fan-out HTTP real
(leaderboard/achievements + service-to-service auth) é a próxima feature.
O allocator é **mock** (pool fixo em memória); o Agones real entra no deploy
(ADR-0011/0012).
