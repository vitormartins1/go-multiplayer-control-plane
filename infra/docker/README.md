# Stack local (Docker Compose)

Sobe todos os serviços + bancos/storage + observabilidade num comando. É o ambiente onde se desenvolve e valida tudo (estratégia local-first; deploy AWS fica para o fim).

```bash
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

### Storage
| Recurso | Porta | Credenciais |
|---|---|---|
| identity-postgres | 5432 | dev / dev (DB `identity`) |
| achievements-postgres | 5433 | dev / dev (DB `achievements`) |
| player-data-postgres | 5434 | dev / dev (DB `player_data`) |
| redis | 6379 | — |
| minio | 9000 (API), 9001 (console) | minioadmin / minioadmin |

### Observabilidade
| Ferramenta | Porta | Notas |
|---|---|---|
| Grafana | 3000 | login anônimo (admin); dashboards provisionados |
| Prometheus | 9090 | Status → Targets para ver os scrapes |
| Tempo | 3200 (query), 4317 (OTLP gRPC) | traces |
| postgres-exporters | 9187 / 9188 / 9189 | identity / achievements / player-data |
| redis-exporter | 9121 | leaderboard |
| (MinIO métricas) | 9000 `/minio/v2/metrics/cluster` | nativo, sem exporter |

## Convenção (por serviço novo)

`<svc>-service` (HTTP/gRPC) · `<svc>-postgres` + `<svc>-postgres-exporter` (se Postgres) · DB `<svc>`. Portas: serviços 808x, Postgres 543x, exporters 918x. Ver o checklist em [docs/OBSERVABILITY.md](../../docs/OBSERVABILITY.md).

## Observabilidade — o que olhar

No **Grafana** (http://localhost:3000), pasta `game-backend`:
- **Service Overview (RED)** — RPS/erros/latência por serviço (`$job`), goroutines/heap/GC.
- **PostgreSQL** / **Redis** / **MinIO** — saúde de cada storage.
- **Tracing (APM)** — service graph (mapa de dependências) + RED derivado dos traces.

Ver um trace em waterfall: **Explore → Tempo** → TraceQL, ex. `{ resource.service.name = "identity-service" }`.

Conceitos e detalhes: [docs/OBSERVABILITY.md](../../docs/OBSERVABILITY.md).

## Notas

- Os bancos recriam o schema na primeira subida (migrations via `docker-entrypoint-initdb.d`); para reaplicar migrations novas, recrie o volume: `docker compose up -d --force-recreate --renew-anon-volumes <svc>-postgres`.
- Após editar provisioning do Grafana (datasources/dashboards), `docker compose restart grafana` (ele só relê no boot).
