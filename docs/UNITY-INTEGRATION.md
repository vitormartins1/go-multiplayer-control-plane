# Integração Unity ↔ Backend Go — guia genérico

> Setup **genérico** de um data plane Unity contra o control plane Go. O **handshake concreto e os claims do connect token são por arquétipo** — a fonte de verdade é o contrato de cada um:
> - `queue_arena` → [`game-models/queue_arena-dataplane-contract.md`](game-models/queue_arena-dataplane-contract.md)
> - `host_world_dropin` → [`game-models/host_world_dropin-dataplane-contract.md`](game-models/host_world_dropin-dataplane-contract.md)
> - `battle_royale` → [`game-models/battle_royale-dataplane-contract.md`](game-models/battle_royale-dataplane-contract.md)
> - `coop_mission` → [`game-models/coop_mission-dataplane-contract.md`](game-models/coop_mission-dataplane-contract.md)
>
> Este doc guarda só o que é **comum a todos os arquétipos** (requisitos do projeto Unity, Agones, canais). Topologia de repos: [`REPO-TOPOLOGY.md`](REPO-TOPOLOGY.md) (um `go-multiplayer-control-plane` + N `unity-<archetype>-data-plane`).

## Estado atual por arquétipo (2026-07-11)

Os 4 arquétipos estão **completos no control plane** (ADR-0023), mas o data plane Unity é cross-repo e evolui em ritmo próprio — dois estão de pé, dois ainda não:

| Arquétipo | Data plane Unity | Estado |
|---|---|---|
| `queue_arena` | `unity-queue-arena-data-plane` (ex-`go-game-dataplane`) | **Rodando de verdade** — Fases 0-7: Dedicated Server ARM64 orquestrado por Agones (k3d local → homelab Pi 5), handshake completo `login → FindMatch → session aloca via Agones → connect token Ed25519 → partida → ReportResult`. É o exemplo canônico. **Tem dívida de contrato pendente** — ver seção abaixo. |
| `host_world_dropin` | `unity-host-world-dropin-data-plane` | ✅ **Fases 0–5 done** (2026-07-11) — mundo do host, phantoms entrando no meio da sessão por papel, invasão + quadro de sinais via matchmaking (incluindo bypass por senha), combate com boss e ciclo de vida completo (host morto, boss derrotado, mundo fecha), **e2e orquestrado k3d→Pi 5 + carga real** (8 mundos/24 processos, GameServers reciclados, fan-out confirmado ponta a ponta). Falta só **Fase 6+** (refinamento: senha/pools por região/reconexão — backlog, sem data fixa). |
| `battle_royale` | (nenhum ainda) | ⬜ Repo ainda não criado. Contrato pronto. |
| `coop_mission` | (nenhum ainda) | ⬜ Repo ainda não criado. Contrato pronto. |

## ⚠️ Dívida de contrato no `queue_arena` — prioridade Alta

O `ReportResult` mudou na spec `session/009` (resultado polimórfico, ADR-0023): o formato antigo `PlayerResult{player_id, score, won bool}` foi substituído por `win_loss_draw{players: [PlayerOutcome{player_id, team_id, outcome string, score, stats}]}` (`outcome` = `"win"|"loss"|"draw"`, permite empate explícito; `stats` carrega as métricas declaradas em `scoringStats`). **O `unity-queue-arena-data-plane` ainda reporta o formato antigo** — o contrato já quebrou, não é uma migração cosmética. Detalhe completo (5 mudanças necessárias, priorizadas) em [`queue_arena-dataplane-contract.md`](game-models/queue_arena-dataplane-contract.md#mudanças-necessárias-no-go-game-dataplane-queue_arena). Isso **não bloqueia o control plane** (os serviços Go funcionam e são testados isoladamente) — é trabalho pendente só do lado Unity.

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

Depende do arquétipo — a tabela concreta está em cada contrato (`…-dataplane-contract.md`). Em comum: gerar os stubs C# do `.proto`, ler o claim do token no connection approval, e reportar ao backend (resultado terminal no `queue_arena`/colocação no `battle_royale`/desfecho coletivo no `coop_mission`; eventos de participação no `host_world_dropin`). O `unity-queue-arena-data-plane` e o `unity-host-world-dropin-data-plane` são os dois exemplos de referência já rodando (ver "Estado atual por arquétipo" no topo) — `battle_royale`/`coop_mission` ainda não têm repo.
