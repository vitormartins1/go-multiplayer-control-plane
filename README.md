# game-backend

Monorepo de microsserviços em **Go** para backend de jogos multiplayer.

## Tese

Reimplementar em Go, sobre infraestrutura própria e barata, os serviços de backend que a **AWS** (GameKit, GameLift) e a **Unity** vendem prontos para jogos — e comparar **custo, performance e eficiência** com a oferta gerenciada. Dois objetivos: aprender Go de forma séria e construir infra reutilizável para jogos próprios.

O realtime de gameplay **não** é feito em Go: ele roda no **Unity Dedicated Server** (data plane, repo separado). Go cuida do **control plane** — auth, matchmaking, sessão, persistência, leaderboard, achievements, analytics — como o GameLift faz: orquestra, não simula.

## Serviços (control plane)

Cada um reimplementa um equivalente AWS e forma um par de comparação. O `identity` é a fundação de auth (ADR-0009); os demais validam o JWT que ele emite. Todos nascem instrumentados (métricas + tracing). Veja o README de cada serviço para detalhes.

| Serviço | Equivalente AWS | Protocolo | Storage | Porta(s) host | Status |
|---|---|---|---|---|---|
| [identity](services/identity-service) | GameKit Identity (Cognito) | REST | Postgres | 8081 | ✅ |
| [achievements](services/achievements-service) | GameKit Achievements | REST | Postgres | 8082 | ✅ |
| [player-data](services/player-data-service) | GameKit User Gameplay Data | REST | Postgres (JSONB) | 8083 | ✅ |
| [cloud-save](services/cloud-save-service) | GameKit Game State Saving | REST | MinIO / S3 | 8084 | ✅ |
| [leaderboard](services/leaderboard-service) | ref. DynamoDB/Redis | REST | Redis (sorted set) | 8085 | ✅ |
| [matchmaking](services/matchmaking-service) | GameLift FlexMatch | gRPC | memória | 50051 (gRPC), 8086 (métricas) | ✅ |
| [session](services/session-service) | GameLift (orquestração) | gRPC | memória | — | 🚧 em desenvolvimento |
| analytics | Game Analytics Pipeline | — | — | — | ⬜ planejado |

O **polyglot persistence** (ADR-0015) é proposital: cada domínio usa o storage que seu padrão de acesso pede — Postgres (CRUD), MinIO (blob), Redis (ranking), memória (fila efêmera).

## Stack

Go 1.22+ · Hexagonal · REST (`net/http`) + gRPC · PostgreSQL (`pgx/v5`) · Terraform + LocalStack · AWS (compute por serviço: Lambda/Fargate/ECS/EC2/EKS) · Prometheus + Grafana + OpenTelemetry · GitHub Actions

## Metodologia

**Spec-Driven Development (SDD):** `spec.md → plan.md → tasks.md → código`. A spec é a fonte de verdade.

## Documentação

| Documento | Conteúdo |
|---|---|
| [docs/ROADMAP.md](docs/ROADMAP.md) | Roadmap de execução e fases de estudo |
| [docs/FEATURE-MAP.md](docs/FEATURE-MAP.md) | Mapa de features de alto nível dos serviços |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Visão consolidada da arquitetura |
| [docs/GO-LEARNINGS.md](docs/GO-LEARNINGS.md) | Aprendizados de Go (exemplos comentados) |
| [docs/UNITY-INTEGRATION.md](docs/UNITY-INTEGRATION.md) | Guia genérico de integração Unity ↔ backend (rascunho) |
| [docs/adr/README.md](docs/adr/README.md) | Índice de Architecture Decision Records |
| [docs/sdd/SDD-GUIDE.md](docs/sdd/SDD-GUIDE.md) | Guia da metodologia SDD |
| [docs/GIT-WORKFLOW.md](docs/GIT-WORKFLOW.md) | Padrão de branches e commits (ADR-0013) |
| [docs/OBSERVABILITY.md](docs/OBSERVABILITY.md) | Observabilidade local (pilares, dashboards, tracing) |
| [infra/docker/README.md](infra/docker/README.md) | Stack local: como subir, mapa de portas, dashboards |
| [docs/adr/ADR-GUIDE.md](docs/adr/ADR-GUIDE.md) | Guia para criação de ADRs |

## Status

**6 serviços implementados** (identity, achievements, player-data, cloud-save, leaderboard, matchmaking) e o **session em desenvolvimento**. Decisões registradas em **ADRs 0001–0015**. Tudo roda localmente via Docker Compose, com observabilidade completa (Prometheus + Grafana + Tempo) e dashboards provisionados. Deploy em AWS (Terraform) fica para o fim — a prioridade é validar o máximo localmente.

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

# Teste manual: arquivos .http por serviço (HTTP Client do GoLand / REST Client do VS Code);
# matchmaking é gRPC -> services/matchmaking-service/api/README.md
```
