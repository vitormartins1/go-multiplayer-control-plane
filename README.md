# game-backend

Monorepo de microsserviços em **Go** para backend de jogos multiplayer.

## Tese

Reimplementar em Go, sobre infraestrutura própria e barata, os serviços de backend que a **AWS** (GameKit, GameLift) e a **Unity** vendem prontos para jogos — e comparar **custo, performance e eficiência** com a oferta gerenciada. Dois objetivos: aprender Go de forma séria e construir infra reutilizável para jogos próprios.

O realtime de gameplay **não** é feito em Go: ele roda no **Unity Dedicated Server** (data plane, repo separado). Go cuida do **control plane** — auth, matchmaking, sessão, persistência, leaderboard, achievements, analytics — como o GameLift faz: orquestra, não simula.

## Serviços (control plane)

`identity` · `achievements` · `player-data` · `cloud-save` · `leaderboard` · `analytics` · `matchmaking` · `session`

Cada um reimplementa um equivalente AWS e forma um par de comparação. Desenvolvidos um por vez, em ordem de complexidade crescente.

## Stack

Go 1.22+ · Hexagonal · REST (`net/http`) + gRPC · PostgreSQL (`pgx/v5`) · Terraform + LocalStack · AWS (compute por serviço: Lambda/Fargate/ECS/EC2/EKS) · Prometheus + Grafana + OpenTelemetry · GitHub Actions

## Metodologia

**Spec-Driven Development (SDD):** `spec.md → plan.md → tasks.md → código`. A spec é a fonte de verdade.

## Documentação

| Documento | Conteúdo |
|---|---|
| [docs/ROADMAP.md](docs/ROADMAP.md) | Roadmap de execução e fases de estudo |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Visão consolidada da arquitetura |
| [docs/GO-LEARNINGS.md](docs/GO-LEARNINGS.md) | Aprendizados de Go (exemplos comentados) |
| [docs/UNITY-INTEGRATION.md](docs/UNITY-INTEGRATION.md) | Guia genérico de integração Unity ↔ backend (rascunho) |
| [docs/adr/README.md](docs/adr/README.md) | Índice de Architecture Decision Records |
| [docs/sdd/SDD-GUIDE.md](docs/sdd/SDD-GUIDE.md) | Guia da metodologia SDD |
| [docs/adr/ADR-GUIDE.md](docs/adr/ADR-GUIDE.md) | Guia para criação de ADRs |

## Status

Em fundação: documentação e decisões arquiteturais (ADRs 0001–0011) estabelecidas. Implementação dos serviços ainda não iniciada.

## Comandos

```bash
# Ambiente local completo
docker compose -f infra/docker/docker-compose.yml up

# Um serviço (ex: identity)
cd services/identity-service && go run ./cmd/server/

# Testes com race detector
go test ./... -race -cover
```
