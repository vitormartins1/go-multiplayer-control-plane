# Integração Unity ↔ Backend Go — guia genérico

> Como integrar um jogo Unity com os serviços de backend em Go deste projeto.
> Escrito de forma **genérica**: serve de **template reutilizável** para qualquer jogo, não só o jogo de estudo.
> **Status: rascunho** — evolui conforme os serviços são implementados. Itens marcados `(TODO)` dependem de código que ainda não existe.

---

## Visão geral

Recap (detalhes em [ARCHITECTURE.md](ARCHITECTURE.md)):

- **Control plane (Go, este repo):** identity, matchmaking, session, leaderboard, achievements, player-data, cloud-save, analytics.
- **Data plane (Unity Dedicated Server):** simulação realtime da partida.
- **2 repos:** `go-game-backend` + `game-unity` (client e server são o mesmo projeto Unity, em 2 build targets).
- **Go não entra no loop realtime** — só no control plane e em chamadas fora do caminho de tick. Ver ARCHITECTURE "Onde o Go entra".

## Canais de comunicação

| Canal | Protocolo | Quando |
|---|---|---|
| Client → backend | HTTP/gRPC | login, matchmaking, leaderboard (fora de partida) |
| Client ↔ Dedicated Server | UDP (Unity Transport) | gameplay realtime |
| Dedicated Server → backend | HTTP/gRPC | valida connect token, heartbeat, resultado |

## Requisitos do projeto Unity

- Unity 6.x, **Netcode for GameObjects** + **Unity Transport**.
- Dois build targets: **Client** (com gráficos) e **Dedicated Server** (Linux headless, **ARM64** para o homelab).
- Server build **containerizado** (Docker) para deploy.
- `(TODO)` Cliente HTTP/gRPC no Unity para falar com o backend Go (gerado do `.proto` compartilhado).
- **Agones C# SDK** no server build (orquestração desde o início — ADR-0012).

## Infra / deploy

- **Control plane:** compute por serviço (Lambda/Fargate/...), ver ADR-0005.
- **Game servers:** container do Unity headless. Espectro (ver ARCHITECTURE §7):
  - **Local/dev:** rodar o container localmente (ou no docker-compose).
  - **1 VPS/EC2 com Docker:** bom para testar com amigos, custo baixo.
  - **Escala:** **Agones em EKS** (fleet + allocation).

## Contrato de integração (handshake)

Visão de alto nível; endpoints/proto concretos são `(TODO)` conforme os serviços nascem.

1. Client → **identity**: login → recebe JWT.
2. Client → **matchmaking**: pede partida (ticket).
3. **matchmaking** → **session**: partida formada.
4. **session** aloca um game server e devolve `{ serverAddress, port, connect token }`.
5. Client conecta no **Dedicated Server** com o token.
6. Dedicated Server **valida o token** contra a identity/session.
7. Fim da partida: Dedicated Server → backend reporta **resultado** → leaderboard/achievements/analytics.

## Como o Agones se encaixa (orquestração de game servers — ADR-0012)

- Agones é o orquestrador **desde o início**. Roda em **k3s (homelab, Raspberry Pi 5 ARM64)** no dev e em **EKS sob demanda** na nuvem. Os game servers rodam como `GameServer` dentro de um `Fleet`.
- O **server build (Unity)** é compilado para **Linux ARM64** (Unity 6) e usa o **Agones SDK (C#)**: `Ready()` no boot, `Health()` periódico, `Allocate()`/`Shutdown()`.
- O **session-service (Go)** chama a **API de Allocation do Agones** (cliente gRPC) para obter um server `Ready` — não reimplementa o fleet.

## TODO (conforme a implementação avança)

- [ ] Definir e versionar os contratos `.proto` (matchmaking, session).
- [ ] Documentar os endpoints concretos de cada serviço.
- [ ] Exemplo de cliente Unity (C#) chamando identity/matchmaking.
- [ ] Dockerfile de referência do Unity Dedicated Server.
- [ ] Manifest de `Fleet` de referência do Agones.
- [ ] Jogo de exemplo mínimo demonstrando as features (ver task de setup do repo Unity).
