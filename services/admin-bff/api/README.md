# admin-bff — testando a API

O `admin-bff` é o back-end-for-frontend do painel de administração (ADR-0016):
agregador **read-only** que lê o Prometheus e a API admin do achievements e
expõe uma API HTTP/JSON para o `admin-ui`. **Sem auth** no MVP (interno/local).

```bash
# Requer a stack no ar:
#   docker compose -f infra/docker/docker-compose.yml up
# admin-bff em http://localhost:8088

# Topologia + saúde dos serviços (status up/down de up{job=...})
curl -s localhost:8088/api/topology | jq
# -> { "nodes":[{"id":"session-service","label":"Session","up":true}, ...],
#      "edges":[{"from":"matchmaking-service","to":"session-service","label":"provisiona sessão"}, ...],
#      "sourceAvailable": true }
# Prometheus fora -> nós com "up": null e "sourceAvailable": false (degrada, 200)

# Contadores ao vivo (do Prometheus)
curl -s localhost:8088/api/metrics/live | jq
# -> { "queueSize":0, "activeSessions":0, "matchesTotal":0, "matchesFailed":0 }
# Prometheus fora -> 503

# Métricas de um serviço (saúde + RED + runtime) — alimenta a página de domínio
curl -s localhost:8088/api/services/identity-service/metrics | jq
# -> { "id":"identity-service","up":true,"requestRate":..,"errorRate":..,
#      "latencyP95":..,"goroutines":..,"memoryBytes":.. }
# serviços gRPC (matchmaking/session) não têm RED HTTP -> esses campos vêm null
# id desconhecido -> 404 | Prometheus fora -> 503

# Lista de usuários (User base) — via identity /admin/players
curl -s "localhost:8088/api/players?limit=50&offset=0" | jq
# -> { "total": <int>, "players": [ {"id","handle","createdAt"} ] }
# identity fora -> 502

# Achievements de um jogador (via /admin do achievements-service)
curl -s localhost:8088/api/players/PLAYER_ID/achievements | jq
# PLAYER_ID = id da conta (sub do JWT). achievements fora -> 502

# saúde / métricas do próprio BFF
curl -s localhost:8088/healthz
curl -s localhost:8088/metrics | grep http_
```

O trace de cada request segue até as fontes (Prometheus/achievements) no Tempo
(ADR-0014). O painel de **jogadores** (listar/contar) e as **ações admin** são
iteração 2 (ver ADR-0016).
