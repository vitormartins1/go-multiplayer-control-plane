# go-multiplayer-control-plane

Monorepo de microsserviços em **Go** para o **control plane** de jogos multiplayer.

> Nome de repo decidido (rename ainda não executado): **`go-multiplayer-control-plane`**. Ver [`docs/REPO-TOPOLOGY.md`](docs/REPO-TOPOLOGY.md).

## Tese

Reimplementar em Go, sobre infraestrutura própria e barata, os serviços de backend que a **AWS** (GameKit, GameLift) e a **Unity** vendem prontos — e comparar **custo, performance e eficiência** com a oferta gerenciada. Dois objetivos: aprender Go de forma séria e construir infra reutilizável para jogos próprios. (ADR-0008)

O realtime de gameplay **não** é Go: roda no **Unity Dedicated Server** (data plane, repos separados por arquétipo — ver topologia). O Go cuida do **control plane** — auth, matchmaking, sessão, persistência, leaderboard, achievements, analytics — como o GameLift: orquestra, não simula. (ADR-0010)

## Multiplayer por arquétipo (ADR-0023)

O control plane é **agnóstico de tipo de jogo**: cada jogo declara no game-registry um **perfil de multiplayer** (arquétipo + parâmetros), e matchmaking/session/fan-out se pré-configuram a partir dele. A generalização é dirigida por **engenharia reversa de jogos reais** em [`docs/game-models/`](docs/game-models/README.md).

| Arquétipo | Forma | Estado (control plane) | Data plane Unity |
|---|---|---|---|
| `queue_arena` | partida simétrica FFA/NvN, resultado win/loss/draw + stats | ✅ completo | ✅ rodando de verdade (`unity-queue-arena-data-plane`) |
| `host_world_dropin` | Souls-like: mundo persistente de um host, phantoms co-op/invasor por papel, resultado por evento | ✅ completo | ✅ rodando de verdade (`unity-host-world-dropin-data-plane`, Fases 0–5 done, e2e k3d→Pi 5 + carga) |
| `battle_royale`, `coop_mission` | colocação ordinal / resultado coletivo | ✅ completo | ⬜ repo Unity ainda não criado |

Estado por fatia: [`docs/game-models/README.md`](docs/game-models/README.md) → "Estado da implementação".

## Serviços (control plane)

Cada um reimplementa um equivalente AWS e forma um par de comparação. Detalhes: README de cada serviço e [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) (mapa canônico).

| Serviço | Equivalente AWS | Protocolo | Storage | Status |
|---|---|---|---|---|
| [identity](services/identity-service) | GameKit Identity (Cognito) | REST | Postgres | ✅ |
| [game-registry](services/game-registry) | (sem equivalente 1:1 — fonte de jogos multi-tenant, ADR-0017) | REST | Postgres | ✅ |
| [achievements](services/achievements-service) | GameKit Achievements | REST + consumer SQS | Postgres | ✅ |
| [player-data](services/player-data-service) | GameKit User Gameplay Data | REST | Postgres (JSONB) | ✅ |
| [cloud-save](services/cloud-save-service) | GameKit Game State Saving | REST | MinIO / S3 | ✅ |
| [leaderboard](services/leaderboard-service) | ref. DynamoDB/Redis | REST + consumer SQS | Redis (sorted set) | ✅ |
| [matchmaking](services/matchmaking-service) | GameLift FlexMatch | gRPC | memória | ✅ |
| [session](services/session-service) | GameLift (orquestração) | gRPC | memória | ✅ |
| [analytics](services/analytics-service) | Game Analytics Pipeline | consumer SQS | Postgres | ✅ |
| [admin-bff](services/admin-bff) + [admin-ui](services/admin-ui) | painel de operação | REST + React | — | ✅ |

Fan-out durável do resultado via **SNS→SQS** (ADR-0020): o session publica no tópico `match-results`; achievements/leaderboard/analytics são consumidores idempotentes. **Polyglot persistence** proposital (ADR-0015): cada domínio usa o storage que seu padrão pede.

## Stack

Go 1.22+ · Hexagonal · REST (`net/http`) + gRPC · PostgreSQL (`pgx/v5`) · Redis · Terraform + LocalStack · AWS (compute por serviço) · Prometheus + Grafana + OpenTelemetry (Tempo) · GitHub Actions

## Metodologia

**Spec-Driven Development (SDD)** em dois níveis por tamanho da fatia — completo (`spec → plan → tasks → código`) para fatias substanciais, enxuto (spec curta) para aditivas. A spec é a fonte de verdade. Ver [`docs/sdd/SDD-GUIDE.md`](docs/sdd/SDD-GUIDE.md).

## Documentação

| Documento | Conteúdo |
|---|---|
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | **Panorama canônico** da arquitetura (control/data plane, serviços, hexagonal, infra) |
| [docs/game-models/README.md](docs/game-models/README.md) | Taxonomia de arquétipos + estado de implementação por fatia |
| [docs/REPO-TOPOLOGY.md](docs/REPO-TOPOLOGY.md) | Control plane + data planes por arquétipo (nomenclatura de repos) |
| [docs/GO-LEARNINGS.md](docs/GO-LEARNINGS.md) | Aprendizados de Go (exemplos comentados) |
| [docs/NEXT-STEPS.md](docs/NEXT-STEPS.md) | Handoff — estado atual e backlog priorizado |
| [docs/adr/README.md](docs/adr/README.md) | Índice de ADRs (0001–0029) |
| [docs/sdd/SDD-GUIDE.md](docs/sdd/SDD-GUIDE.md) · [docs/GIT-WORKFLOW.md](docs/GIT-WORKFLOW.md) | Metodologia SDD · branches/commits |
| [docs/OBSERVABILITY.md](docs/OBSERVABILITY.md) · [infra/docker/README.md](infra/docker/README.md) | Observabilidade local · stack local (portas, dashboards) |
| [docs/ROADMAP.md](docs/ROADMAP.md) · [docs/FEATURE-MAP.md](docs/FEATURE-MAP.md) | Fases de estudo/FinOps · índice de features por serviço |

## Status

**Trilha de serviços 100% construída** (10 serviços Go + painel admin), rodando localmente via Docker Compose com observabilidade completa (Prometheus + Grafana + Tempo). **4 arquétipos de multiplayer completos** no control plane (`queue_arena`, `host_world_dropin`, `battle_royale`, `coop_mission`) — cross-repo, `queue_arena` e `host_world_dropin` já têm data plane Unity rodando de verdade, `battle_royale`/`coop_mission` ainda sem repo Unity. Decisões em **ADRs 0001–0029**. Deploy em AWS (Terraform) fica para o fim — a prioridade é validar o máximo localmente.

## Comandos

```bash
# Ambiente local completo (todos os serviços + observabilidade)
docker compose -f infra/docker/docker-compose.yml up --build
#   ver infra/docker/README.md para o mapa de portas e dashboards

# Um serviço isolado (ex: identity)
cd services/identity-service && go run ./cmd/server/

# Testes unitários (com race detector)
go test ./... -race -cover

# Testes de integração (testcontainers; requer Docker)
go test ./... -tags=integration

# Teste manual: coleção .http por serviço; matchmaking/session são gRPC (grpcurl / cliente de teste)
```
