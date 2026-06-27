# Architecture Decision Records

Índice dos ADRs do projeto. Ver `ADR-GUIDE.md` para o processo e `0000-template.md` para o template.

| # | Título | Status | Data | Serviço |
|---|---|---|---|---|
| [0001](0001-monorepo-structure.md) | Monorepo Go iniciando como módulo único | Accepted | 2026-06-27 | todos |
| [0002](0002-hexagonal-architecture.md) | Hexagonal Architecture idiomática em Go | Accepted | 2026-06-27 | todos |
| [0003](0003-grpc-matchmaking.md) | gRPC com streaming para matchmaking e sessão | Accepted | 2026-06-27 | matchmaking, session |
| [0004](0004-terraform-localstack.md) | Terraform + LocalStack (local) / AWS (prod) | Accepted | 2026-06-27 | infra, todos |
| [0005](0005-compute-selection-framework.md) | Seleção de compute por serviço (diferida) | Accepted | 2026-06-27 | todos, infra |
| [0006](0006-prometheus-grafana.md) | Prometheus + Grafana + OpenTelemetry | Accepted | 2026-06-27 | todos, infra |
| [0007](0007-sdd-methodology.md) | Spec-Driven Development como metodologia | Accepted | 2026-06-27 | todos |
| [0008](0008-generic-compute-vs-aws-game-services.md) | Reimplementar serviços de jogo AWS em Go sobre compute genérico | Accepted | 2026-06-27 | todos |
| [0009](0009-gamekit-faithful-decomposition.md) | Decomposição fiel ao GameKit (identity como fundação) | Accepted | 2026-06-27 | identity, achievements, player-data, cloud-save |
| [0010](0010-control-plane-data-plane.md) | Control plane (Go) vs data plane (Unity Dedicated Server) | Accepted | 2026-06-27 | matchmaking, session, Unity |
| [0011](0011-unity-netcode-agones.md) | Unity Netcode for GameObjects + Agones/EKS, sem serviços pagos | Accepted | 2026-06-27 | data plane, infra |
