# Roadmap & fases de estudo — game-backend

> **Panorama da arquitetura:** [`ARCHITECTURE.md`](ARCHITECTURE.md) (canônico — tese, control/data plane, mapa de serviços, hexagonal, infra, observabilidade). **Estado por arquétipo:** [`game-models/README.md`](game-models/README.md). **Backlog atual + handoff:** [`NEXT-STEPS.md`](NEXT-STEPS.md).
>
> Este doc guarda o que é só dele: a **trilha de estudo em fases**, **FinOps** e **CI/CD**. Foi enxugado (2026-07-07) — as seções de panorama que duplicavam o ARCHITECTURE saíram.

## Visão em uma frase

Reimplementar em Go, sobre infra própria e barata, os serviços de backend que a AWS (GameKit/GameLift) e a Unity vendem prontos — control plane agnóstico de tipo de jogo (perfil por arquétipo, ADR-0023) — e **comparar custo/performance** com a oferta gerenciada (ADR-0008). O realtime roda no Unity Dedicated Server (data plane, repos por arquétipo — `REPO-TOPOLOGY.md`).

## Trilha de estudo — fases (✅ construída)

A trilha foi percorrida em ordem de complexidade crescente; **toda a trilha de serviços está construída** (ver estado real em `NEXT-STEPS.md`). Mantida aqui como registro do caminho de aprendizado.

| Fase | Foco (aprendizado Go) | Estado |
|---|---|---|
| 0 | Fundação de docs/ADRs + scaffold do monorepo | ✅ |
| 1 | Fundamentos Go (structs, interfaces, errors, testes) | ✅ |
| 2 | identity-service + **hexagonal** (domain/service/repository/transport) | ✅ |
| 3 | Terraform + LocalStack + primeiro compute | ✅ (local; prod AWS = backlog) |
| 4 | achievements-service — domínio rico, auth entre serviços | ✅ |
| 5 | player-data-service — CRUD, modelagem de dados | ✅ |
| 6 | cloud-save-service — blob versionado (S3/MinIO) | ✅ |
| 7 | leaderboard-service — Redis sorted sets, always-on | ✅ |
| 8 | matchmaking-service — protobuf/gRPC streaming, pool de tickets concorrente | ✅ |
| 9 | analytics-service — worker pool (goroutines/channels/context) | ✅ |
| 10 | session-service + realtime (control/data plane, Agones) — capstone | ✅ (control plane; data plane cross-repo) |
| 11 | **Comparação Go vs. AWS** (entregável de tese: custo/latência/throughput) | ⬜ pendente (precisa do deploy prod) |

Além da trilha, o projeto generalizou o multiplayer por **arquétipo** (ADR-0023): `queue_arena` e `host_world_dropin` completos no control plane; `battle_royale`/`coop_mission` planejados. Fases seguintes vivem em `NEXT-STEPS.md` (backlog priorizado) e nos roadmaps por arquétipo em `game-models/`.

## Práticas transversais — desde o serviço 1

- **Observabilidade de nascença** (ADR-0006): cada serviço sobe instrumentado (slog JSON + Prometheus + OTel/Tempo) e entra na stack local; não é fase no fim. Ritual e nomenclatura por serviço em `OBSERVABILITY.md`.
- **Testes em camadas:** unit (`testify`, desde o serviço 1, cobre os casos da spec) · integração (`testcontainers-go`, banco real, gated por `-tags=integration`) · e2e de sessão real (meta futura) · IaC testada (Terratest, quando a infra avançar).

## CI/CD — GitHub Actions

- **CI por serviço** (paralelo), filtrado por `services/<nome>/**`: `go test ./... -race -cover` + `go build`. Testes de integração exigem Docker no runner (testcontainers).
- **CD seletivo:** só deploya o serviço que mudou (`dorny/paths-filter`), respeitando o compute escolhido para cada um (ADR-0005).

## FinOps — controle de custo

Budget AWS via Terraform, alerta em 80% de **$40/mês**. Serviços stateless em Lambda tendem a **abaixar** o custo vs. tudo-em-Fargate (free tier + escala a zero). Estimativa de partida: **~$20–37/mês**. A coluna de custo por serviço alimenta a **comparação com o equivalente AWS gerenciado** (a tese).

**Kubernetes/Agones fora da AWS:** EKS custa ~$73/mês só o control plane — por isso k8s+Agones rodam num **homelab (k3s no Raspberry Pi 5 16GB)** no dev, e o EKS é **efêmero** (sobe/destrói via Terraform) só para testes de nuvem. (ADR-0012)

## Referências

| Recurso | URL |
|---|---|
| Tour of Go · Go by Example · Effective Go | tour.golang.org · gobyexample.com · go.dev/doc/effective_go |
| AWS SDK Go v2 · gRPC Go | pkg.go.dev/github.com/aws/aws-sdk-go-v2 · grpc.io/docs/languages/go |
| AWS GameLift FlexMatch · GameKit | docs.aws.amazon.com/gameliftservers · docs.aws.amazon.com/gamekit |
| LocalStack · Terraform AWS Provider | docs.localstack.cloud · registry.terraform.io/providers/hashicorp/aws |
| Prometheus Go · OpenTelemetry Go | prometheus.io/docs/guides/go-application · opentelemetry.io/docs/languages/go |
| Agones | agones.dev |
| SDD (GitHub Spec Kit) | github.com/github/spec-kit |
