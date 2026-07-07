# Integração Unity ↔ Backend Go — guia genérico

> Setup **genérico** de um data plane Unity contra o control plane Go. O **handshake concreto e os claims do connect token são por arquétipo** — a fonte de verdade é o contrato de cada um:
> - `queue_arena` → [`game-models/queue_arena-dataplane-contract.md`](game-models/queue_arena-dataplane-contract.md)
> - `host_world_dropin` → [`game-models/host_world_dropin-dataplane-contract.md`](game-models/host_world_dropin-dataplane-contract.md)
>
> Este doc guarda só o que é **comum a todos os arquétipos** (requisitos do projeto Unity, Agones, canais). Topologia de repos: [`REPO-TOPOLOGY.md`](REPO-TOPOLOGY.md) (um `go-multiplayer-control-plane` + N `unity-<archetype>-data-plane`).

## Visão geral

- **Control plane (Go, este repo):** identity, matchmaking, session, leaderboard, achievements, player-data, cloud-save, analytics. Agnóstico de arquétipo (orientado por perfil, ADR-0023).
- **Data plane (Unity Dedicated Server):** simulação realtime da partida/mundo. Um repo por arquétipo.
- **Go não entra no loop realtime** — só no control plane e em chamadas fora do caminho de tick (ver [ARCHITECTURE.md](ARCHITECTURE.md) "Onde o Go entra").

## Canais de comunicação

| Canal | Protocolo | Quando |
|---|---|---|
| Client → backend | HTTP/gRPC | login, matchmaking, leaderboard (fora de partida) |
| Client ↔ Dedicated Server | UDP (Unity Transport) | gameplay realtime |
| Dedicated Server → backend | gRPC | valida connect token (offline via JWKS), heartbeat, resultado/eventos |

## Requisitos do projeto Unity (comum a todo arquétipo)

- Unity 6.x, **Netcode for GameObjects** + **Unity Transport**.
- Dois build targets no mesmo projeto: **Client** (com gráficos) e **Dedicated Server** (Linux headless, **ARM64** para o homelab); server build **containerizado**.
- Stub do cliente gRPC gerado do `.proto` compartilhado (`session.v1`, `matchmaking.v1`).
- **Agones C# SDK** no server build (orquestração desde o início — ADR-0012).
- Ler o **connect token** no *connection approval* do NGO: validar offline (JWKS do session) e aplicar o **claim por arquétipo** (`team_id` no queue_arena, `role` no host_world_dropin) no gameplay.

## Agones (orquestração de game servers — ADR-0012)

- Agones orquestra desde o início: **k3s (homelab, Raspberry Pi 5 ARM64)** no dev, **EKS efêmero** na nuvem. Game servers rodam como `GameServer` dentro de um `Fleet`.
- O **server build** usa o Agones SDK (C#): `Ready()` no boot, `Health()` periódico, `Allocate()`/`Shutdown()`.
- O **session-service (Go)** é cliente da **Allocation API** do Agones — não reimplementa o fleet.

## O que cada data plane Unity precisa implementar

Depende do arquétipo — a tabela concreta está em cada contrato (`…-dataplane-contract.md`). Em comum: gerar os stubs C# do `.proto`, ler o claim do token no connection approval, e reportar ao backend (resultado terminal no `queue_arena`; eventos de participação no `host_world_dropin`). O `unity-queue-arena-data-plane` (ex-`go-game-dataplane`) é o exemplo de referência já existente.
