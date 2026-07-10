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
| [0012](0012-agones-from-start.md) | Agones desde o início (homelab k3s / EKS sob demanda) | Accepted | 2026-06-27 | session, data plane, infra |
| [0013](0013-git-branching-strategy.md) | Trunk-based + feature branches + conventional commits | Accepted | 2026-06-27 | todos (processo) |
| [0014](0014-grafana-tempo-tracing.md) | Grafana Tempo + OTLP para distributed tracing | Accepted | 2026-06-28 | todos, infra |
| [0015](0015-database-per-service-polyglot.md) | Database-per-service + polyglot com Postgres default | Accepted | 2026-06-28 | todos, infra |
| [0016](0016-admin-panel-bff.md) | Painel admin — BFF em Go + SPA, agregação via APIs/Prometheus | Accepted | 2026-06-29 | admin-bff, admin-ui, infra |
| [0017](0017-multi-tenant-game-registry.md) | Multi-tenant por jogo — game-registry + dimensão game_id | Accepted | 2026-06-29 | game-registry, achievements, admin-bff, admin-ui, infra |
| [0018](0018-service-to-service-auth.md) | Autenticação serviço-a-serviço (internal API key) | Accepted | 2026-06-29 | todos, infra |
| [0019](0019-game-presence.md) | Presença por jogo (player base por jogo) sobre identidade global | Accepted | 2026-06-30 | game-registry, identity, admin-bff, admin-ui |
| [0020](0020-durable-result-fanout.md) | Fan-out durável do resultado via SNS→SQS (LocalStack local, AWS prod) | Accepted | 2026-07-04 | session, achievements, leaderboard, infra |
| [0021](0021-analytics-ingestion-aggregation.md) | analytics-service ingere o fan-out via worker pool e agrega em Postgres | Accepted | 2026-07-04 | analytics, admin-bff, admin-ui, infra |
| [0022](0022-orphan-session-reconcile-agones.md) | Reconciliação de sessões órfãs contra o Agones (fecha `abandoned`, TTL vira backstop) | Accepted | 2026-07-06 | session |
| [0023](0023-multiplayer-profile.md) | Perfil de multiplayer por jogo — arquétipo + parâmetros orientando o control plane | Accepted | 2026-07-06 | game-registry, matchmaking, session, achievements, leaderboard, analytics, admin-bff, admin-ui, data plane |
| [0024](0024-multiplayer-modes.md) | Modos de jogo — perfis de multiplayer nomeados por jogo + session manifest | Accepted | 2026-07-07 | game-registry, matchmaking, session, achievements, leaderboard, analytics, admin-bff, admin-ui, data plane |
| [0025](0025-game-deletion-soft-delete.md) | Remoção de jogo — soft delete no game-registry (hard purge adiado por evidência) | Accepted | 2026-07-08 | game-registry, admin-bff, admin-ui |
| [0026](0026-agones-fleet-per-game-allocation.md) | Alocação Agones fleet-aware por jogo (Fleet por game_id) | Accepted | 2026-07-09 | session-service, game-registry, admin-bff, admin-ui, data plane |
| [0027](0027-generic-event-rules.md) | Regras de mapeamento evento→achievement-event-type genéricas e data-driven | Accepted | 2026-07-09 | achievements-service, admin-bff, admin-ui |
| [0028](0028-visual-rules-editor-and-rich-engine.md) | Editor visual de regras (canvas low-code) e roadmap do motor de regras rico | Accepted | 2026-07-09 | admin-ui, achievements-service |
| [0029](0029-centralized-agones-infra.md) | Infra Agones centralizada no control plane (bootstrap + fleets); CI/registry por imagem gated por evidência | Proposed | 2026-07-10 | infra, session-service, game-registry, data plane |
