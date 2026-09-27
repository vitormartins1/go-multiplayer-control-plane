# Stack local (Docker Compose)

Sobe todos os serviços + bancos/storage + observabilidade num comando. É o ambiente onde se desenvolve e valida tudo (estratégia local-first; deploy AWS fica para o fim).

```bash
infra/docker/gen-secrets.sh   # uma vez: chaves de dev do identity e do session (o compose falha sem elas)
docker compose -f infra/docker/docker-compose.yml up --build
# subir um subconjunto:
docker compose -f infra/docker/docker-compose.yml up -d identity-service identity-postgres
```

## Mapa de portas (host)

### Serviços
| Serviço | Porta | Protocolo |
|---|---|---|
| identity-service | 8081 | REST |
| achievements-service | 8082 | REST |
| player-data-service | 8083 | REST |
| cloud-save-service | 8084 | REST |
| leaderboard-service | 8085 | REST |
| matchmaking-service | 50051 (gRPC), 8086 (HTTP métricas) | gRPC |
| session-service | 50052 (gRPC), 8087 (HTTP: JWKS, `/admin`, métricas) | gRPC + REST |
| admin-bff | 8088 | REST |
| admin-ui (console) | 8089 | HTTP (nginx) |
| game-registry | 8090 | REST |
| analytics-service | 8091 | REST (`/admin`) |
| achievements-consumer / leaderboard-consumer | — | workers SQS (sem porta publicada) |

### Storage
| Recurso | Porta | Credenciais |
|---|---|---|
| identity-postgres | 5432 | dev / dev (DB `identity`) |
| achievements-postgres | 5433 | dev / dev (DB `achievements`) |
| player-data-postgres | 5434 | dev / dev (DB `player_data`) |
| game-registry-postgres | 5435 | dev / dev (DB `game_registry`) |
| analytics-postgres | 5436 | dev / dev (DB `analytics`) |
| session-postgres | 5437 | dev / dev (DB `session`, histórico de sessões) |
| localstack (SNS/SQS + S3) | 4566 | `test` / `test` — SNS/SQS criados pelo `messaging-provisioner`; bucket `cloud-save` criado pelo serviço (não persiste entre restarts) |
| redis | 6379 | — |

### Observabilidade
| Ferramenta | Porta | Notas |
|---|---|---|
| Grafana | 3000 | login anônimo (admin); dashboards provisionados |
| Prometheus | 9090 | Status → Targets para ver os scrapes |
| Tempo | 3200 (query), 4317 (OTLP gRPC) | traces |
| postgres-exporters | 9187 / 9188 / 9189 / 9190 / 9191 / 9192 | identity / achievements / player-data / game-registry / analytics / session |
| redis-exporter | 9121 | leaderboard |

## Convenção (por serviço novo)

`<svc>-service` (HTTP/gRPC) · `<svc>-postgres` + `<svc>-postgres-exporter` (se Postgres) · DB `<svc>`. Portas: serviços 808x, Postgres 543x, exporters 918x. Ver o checklist em [docs/OBSERVABILITY.md](../../docs/OBSERVABILITY.md).

## Observabilidade — o que olhar

No **Grafana** (http://localhost:3000), pasta `game-backend`:
- **Service Overview (RED)** — RPS/erros/latência por serviço (`$job`), goroutines/heap/GC.
- **PostgreSQL** / **Redis** — saúde de cada storage (o S3 do cloud-save é o LocalStack, sem métricas próprias).
- **Tracing (APM)** — service graph (mapa de dependências) + RED derivado dos traces.

Ver um trace em waterfall: **Explore → Tempo** → TraceQL, ex. `{ resource.service.name = "identity-service" }`.

Conceitos e detalhes: [docs/OBSERVABILITY.md](../../docs/OBSERVABILITY.md).

## Notas

- Todos os serviços de longa duração têm `restart: unless-stopped` (lição do incidente de 2026-08-19); os `/admin` exigem o header `X-Internal-Key: dev-internal-key` (ADR-0018).

- Os bancos recriam o schema na primeira subida (migrations via `docker-entrypoint-initdb.d`); para reaplicar migrations novas, recrie o volume: `docker compose up -d --force-recreate --renew-anon-volumes <svc>-postgres`.
- Após editar provisioning do Grafana (datasources/dashboards), `docker compose restart grafana` (ele só relê no boot).
