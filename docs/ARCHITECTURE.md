# Arquitetura — game-backend

> Visão consolidada da arquitetura do repositório. Aqui está o **panorama e o porquê de alto nível**; cada decisão pontual tem seu **ADR** em `docs/adr/`. O roadmap de execução está em `ROADMAP.md`.

---

## 1. Tese do projeto

Reimplementar em Go, sobre infraestrutura própria e barata, os serviços de backend que a AWS (e a Unity) vendem prontos para jogos — e comparar **custo, performance e eficiência** entre a nossa versão e a oferta gerenciada. Dois objetivos: **aprender Go de forma séria** e **construir infra reutilizável** para jogos próprios. (ADR-0008)

A observabilidade não é enfeite — é o **instrumento de medição** dessa comparação. (ADR-0006)

---

## 2. Topologia: control plane (Go) vs data plane (Unity)

A decisão estrutural mais importante: o realtime de gameplay **não é Go**. Go cuida do *control plane*; o Unity Dedicated Server cuida do *data plane*. É como o GameLift de fato funciona — ele orquestra, não simula. (ADR-0010, ADR-0011)

```mermaid
flowchart TB
    Client["Unity Client"]

    subgraph CP["Control plane — Go (este repo)"]
        ID["identity (auth)"]
        ACH["achievements"]
        PD["player-data"]
        CS["cloud-save"]
        LB["leaderboard"]
        AN["analytics"]
        MM["matchmaking"]
        SS["session-service<br/>sessão · registry · connect token"]
        MM --> SS
    end

    subgraph DP["Data plane — Unity Dedicated Server (repo separado)"]
        GS["Netcode for GameObjects + Unity Transport<br/>input · tick · estado · colisão · snapshots · sync"]
    end

    Client --> ID
    Client --> MM
    SS -->|endpoint + connect token| Client
    Client -->|conecta| GS
    GS -->|valida token| SS
    GS -->|resultado da partida| SS
```

Serviços pagos (Unity Relay/Lobby/Matchmaker/Multiplay, AWS GameLift/GameKit gerenciados) são **substituídos por serviços Go** + game server com IP/porta. Escala de game server: EC2/VPS → ECS-on-EC2 → **Agones/EKS** (open-source). (ADR-0011)

### Onde o Go entra (e onde NÃO entra) no gameplay

Durante a partida em tempo real, **o Go não participa**. A simulação autoritativa (input, tick, física, colisão, estado do mundo) roda inteira no Unity Dedicated Server, falando direto com os clients por UDP. O Go atua **em volta** da partida (control plane) e em chamadas **fora do caminho de tick** (validar token, heartbeat, resultado) — nunca no loop por frame, onde um salto extra só adicionaria latência.

```mermaid
flowchart LR
    Client["Unity Client"]
    Server["Unity Dedicated Server<br/>(headless, autoritativo)"]
    Go["Go backend<br/>(control plane)"]

    Client <-->|"UDP realtime · por tick · SEM Go"| Server
    Client -->|"HTTP/gRPC · login, matchmaking, leaderboard"| Go
    Server -->|"HTTP/gRPC · valida token, heartbeat, resultado"| Go
```

> Nuance: o host realtime ser Unity vale para jogos com física/ação (o server precisa rodar a MESMA simulação do client). Para jogos turn-based/baixa frequência (cartas, tabuleiro), um serviço Go poderia hospedar a lógica da partida diretamente, sem Unity Dedicated Server — matchmaking e session continuam iguais; só muda o "host da partida".

### Repos: 1 control plane + N data planes (por arquétipo)

Client e Dedicated Server de um jogo são **o mesmo projeto Unity** (dois build targets: client gráfico, server headless) — compartilham o código de jogo. Como o backend virou agnóstico de arquétipo (ADR-0023), a topologia é **um control plane, N data planes**: cada arquétipo tem um data plane Unity de referência, em repo próprio, para validar aquele tipo de jogo isoladamente. Contrato via `.proto` + docs de contrato por arquétipo. Nomenclatura decidida (`go-multiplayer-control-plane` + `unity-<archetype>-data-plane`, rename pendente) e mapa de repos em **`docs/REPO-TOPOLOGY.md`**. (ADR-0010, ADR-0011)

---

## 3. Mapa de serviços (control plane) — **canônico**

Cada serviço é a reimplementação de um equivalente AWS e forma um **par de comparação**. Trilha em ordem de complexidade crescente. Este é o **mapa canônico** (README e ROADMAP apontam para cá; não repetir a tabela em outros docs). (ADR-0008, ADR-0009)

| # | Serviço | Equivalente AWS | Papel | Estado |
|---|---|---|---|---|
| 1 | identity-service | GameKit Identity & Auth | auth/JWT próprio; **fundação** dos demais | ✅ |
| 2 | achievements-service | GameKit Achievements | desbloqueio por evento/stat, progresso; consumer SQS | ✅ |
| 3 | player-data-service | GameKit User Gameplay Data | inventário, stats, cross-play | ✅ |
| 4 | cloud-save-service | GameKit Game State Saving | save versionado na nuvem | ✅ |
| 5 | leaderboard-service | ref. arch DynamoDB/Redis | ranking em tempo real; consumer SQS | ✅ |
| 6 | analytics-service | Game Analytics Pipeline | telemetria via worker pool; consumer SQS | ✅ |
| 7 | matchmaking-service | GameLift FlexMatch | fila/tickets + injeção por arquétipo, gRPC streaming | ✅ |
| 8 | session-service | GameLift (orquestração) | ciclo de sessão, registry, connect token, resultado | ✅ |
| — | admin-bff + admin-ui | painel de operação (ADR-0016) | agrega os serviços + Prometheus; React | ✅ |

Os quatro do GameKit (1–4) são **serviços separados, com a identity como fundação de auth** — espelhando o deploy per-feature do GameKit. O compute de cada um é decidido no `plan.md` (ADR-0005), não pré-fixado. (ADR-0009) O fan-out durável do resultado (ADR-0020) liga session → achievements/leaderboard/analytics por **SNS→SQS** idempotente.

Fora de escopo por ora: economy (≈ GameSparks). Lobby entra como serviço próprio se/quando um jogo precisar.

---

## 3a. Multiplayer por arquétipo (ADR-0023) — o control plane é agnóstico de tipo de jogo

A decisão de generalização mais importante depois da topologia: **o control plane não assume um tipo de jogo**. Cada jogo declara no game-registry um **perfil de multiplayer** = um **arquétipo** (a *forma* da sessão/matchmaking/resultado) + parâmetros; matchmaking, session e fan-out **leem o perfil e se pré-configuram**. Um jogo novo de um arquétipo já suportado **não** exige código novo.

A generalização é dirigida por **engenharia reversa de jogos reais** (não por especulação): `docs/game-models/` disseca jogos, extrai o que o backend precisa modelar, e cada gap maduro vira ADR + specs. **Gênero ≠ arquétipo** (ficha 05): vários gêneros (shooter, MOBA, esporte, fighting) colapsam no mesmo arquétipo com parâmetros diferentes; câmera/arte/economia são conteúdo do data plane.

| Arquétipo | Forma | Resultado | Estado |
|---|---|---|---|
| `queue_arena` | partida simétrica formada por fila (FFA / NvN) | win/loss/draw por jogador/time + stats | ✅ completo |
| `host_world_dropin` | mundo persistente de um host; phantoms co-op/invasor entram/saem por papel | por **evento/participante** (não placar terminal) | ✅ completo (H0–H3) |
| `battle_royale` | lobby grande, eliminação progressiva | colocação ordinal (1º..Nº) | ✅ completo (BR0–BR4) |
| `coop_mission` | co-op puro vs. ambiente | coletivo (sucesso/falha por sessão) | ✅ completo (C0–C4) |

Como o perfil orienta cada serviço, o estado por fatia e os roadmaps por arquétipo vivem em **`docs/game-models/README.md`** (fonte única do estado de implementação). Contratos control↔data plane por arquétipo: `docs/game-models/<archetype>-dataplane-contract.md`.

---

## 4. Arquitetura interna e organização do código

- **Monorepo de módulo único** (um `go.mod` na raiz). Cada serviço é binário/imagem Docker independentes; `shared/` é pacote interno. Migração para multi-módulo + `go.work` só com necessidade concreta. (ADR-0001)
- **Hexagonal idiomático** em todos os serviços: `domain / service / repository / transport`. Interfaces declaradas no consumidor; régua anti-interface-pollution. (ADR-0002)
- **gRPC com streaming** para matchmaking/session; **REST** para os serviços de CRUD. (ADR-0003)
- **SDD**: `spec.md → plan.md → tasks.md → código`, spec é fonte de verdade. (ADR-0007)

```mermaid
flowchart TB
    T["transport/ — Adapter de entrada (HTTP/gRPC), só traduz"]
    S["service/ — Core: regras de negócio puras"]
    R["repository/ — Adapter de saída (banco, cache, SQS)"]
    T -->|interface Port de domain/| S
    S -->|interface Port de domain/| R
```

Estrutura de pastas de cada serviço:

```
services/<nome>-service/
  cmd/server/main.go     entrypoint + wire-up
  internal/domain/       tipos puros + interfaces (Ports) — zero deps externas
  internal/service/      regras de negócio (só interfaces de domain)
  internal/repository/   acesso a dados (implementa interfaces de domain)
  internal/transport/    HTTP/gRPC handlers (só traduz)
  migrations/ · specs/
```

---

## 5. Infraestrutura e compute

- **Terraform** como IaC; **LocalStack** no local, **AWS** em prod; state independente por serviço. (ADR-0004)
- **Compute por serviço, decidido na hora de construir** — menu Lambda/Fargate/ECS-EC2/EC2/EKS. CRUD stateless → Lambda; worker always-on → ECS/EC2; streaming/stateful → ECS/EKS; game server realtime → EC2/ECS-EC2/Agones (nunca Fargate). A escolha de banco (gerenciado vs auto-provisionado) segue a mesma lógica diferida. (ADR-0005)

---

## 6. Fluxo de uma partida (arquétipo `queue_arena`)

Este é o fluxo do `queue_arena` (partida formada por fila). Cada arquétipo tem o seu — o `host_world_dropin`, por exemplo, não forma partida: o host **abre um mundo** e phantoms são **injetados por papel** ao longo da vida da sessão, reportando **eventos** em vez de um resultado terminal (ver `docs/game-models/host_world_dropin-dataplane-contract.md`).

```mermaid
sequenceDiagram
    participant C as Unity Client
    participant ID as identity-service
    participant MM as matchmaking-service
    participant SS as session-service
    participant GS as Unity Dedicated Server
    participant BE as Backend services
    C->>ID: login
    ID-->>C: JWT
    C->>MM: Find Match (ticket)
    MM->>SS: sala fechada (com composição de times)
    SS->>GS: aloca / escolhe game server
    SS-->>C: endpoint + connect token (claim team_id)
    C->>GS: conecta (token)
    GS->>ID: valida connect token (offline, JWKS)
    Note over GS: partida roda (realtime)
    GS->>SS: ReportResult (win_loss_draw)
    SS->>BE: fan-out (SNS→SQS): leaderboard, achievements, analytics
```

---

## 7. Alocação de game server (session-service)

Quando o matchmaking fecha uma partida, o `session-service` precisa achar um Dedicated Server livre e saudável para hospedá-la — a reimplementação do *game session placement* do GameLift. Quatro responsabilidades: **registry** (quais servers existem e seu estado), **health** (heartbeat detecta server morto), **allocation** (reivindicar um livre sem corrida) e **reclaim** (devolver ao pool no fim).

Ciclo de vida de um game server:

```mermaid
stateDiagram-v2
    [*] --> Ready: boot + registra
    Ready --> Allocated: AllocateSession (claim atômico)
    Allocated --> Shutdown: partida termina
    Shutdown --> Ready: reinicia limpo
    Ready --> Down: heartbeat perdido
    Allocated --> Down: heartbeat perdido
    Down --> [*]
```

Fluxo de alocação:

```mermaid
sequenceDiagram
    participant GS as Game Server (Unity headless)
    participant SS as session-service
    participant MM as matchmaking-service
    participant C as Unity Client
    GS->>SS: registra (Ready, addr, região, versão)
    loop a cada N s
        GS->>SS: heartbeat
    end
    MM->>SS: AllocateSession(matchId, players, região)
    SS->>SS: claim atômico Ready -> Allocated
    SS->>GS: hospede matchId (jogadores, chave do token)
    SS-->>C: serverAddress + port + connect token
    C->>GS: conecta (token)
    GS->>SS: valida token / confirma
    Note over GS: partida roda
    GS->>SS: matchId concluído (resultado vai ao backend)
    SS->>GS: reciclar -> Ready
```

> O **claim atômico** Ready→Allocated é o ponto de concorrência crítico (duas partidas não podem pegar o mesmo server). Em processo único: `sync.Mutex`. Distribuído (várias instâncias do session-service): update condicional no store (DynamoDB `... IF state=Ready` / Postgres `UPDATE ... WHERE state='Ready' RETURNING`). Detalhado em `docs/GO-LEARNINGS.md` §01.

**Decisão (ADR-0012): Agones desde o início.** Em vez de construir um pool em Go, o `session-service` é **cliente fino da Allocation API do Agones**. O Agones é um projeto open-source que estende o Kubernetes para gerenciar game servers: CRD `GameServer` com o ciclo de vida acima, `Fleet` com buffer de servers `Ready`, API de *Allocation* e autoscaler. É **engine-agnóstico** — empacota-se o Unity Dedicated Server (Linux headless **ARM64**, suportado no Unity 6) num container e integra-se o **Agones SDK (C#)** para reportar Ready/Allocated/Health/Shutdown.

Onde roda (ADR-0012):

| Ambiente | Kubernetes | Custo |
|---|---|---|
| Dev / homelab | **k3s** no Raspberry Pi 5 16GB (ARM64) | grátis (hardware próprio) |
| Cloud (sob demanda) | **EKS** efêmero (sobe/destrói via Terraform) só p/ testes de nuvem | ~$73/mês só ligado → manter efêmero |

> O claim atômico Ready→Allocated passa a ser responsabilidade do Agones, não do session-service. O exemplo em Go (`GO-LEARNINGS.md` §01) permanece como **estudo de concorrência**; a concorrência de produção vive no matchmaking (pool de tickets).

### Connect token

Ao alocar a sessão, o session-service também cunha um **connect token** — o ingresso criptográfico que o jogador apresenta ao Unity Dedicated Server. Ele prova *"este jogador foi autorizado a entrar nesta partida, neste server, dentro de uma janela curta"*, impedindo que alguém conecte direto no IP:porta sem passar pelo matchmaking.

Claims (JWT):

```json
{
  "sub": "player-abc123",     // quem
  "match_id": "match-xyz789", // qual partida
  "server": "10.0.0.5:7777",  // vale só para este server
  "iat": 1719500000,
  "exp": 1719500060,          // ~60s
  "jti": "nonce-uuid"         // anti-reuso
}
```

Assinatura **assimétrica (Ed25519)**: o backend assina com a **chave privada** (nunca sai dele); o game server verifica com a **chave pública**, **localmente, sem round-trip ao backend** a cada conexão. No Netcode for GameObjects, o token viaja no payload de *connection approval*, validado no callback de aprovação do server. Implementação em `GO-LEARNINGS.md` §02.

> **Claims por arquétipo (ADR-0023):** além dos claims base, o token carrega um claim que o data plane lê no *connection approval* para o gameplay — `team_id` no `queue_arena` (lado do jogador na partida NvN) ou `role` no `host_world_dropin` (`host`/`coop`/`invader`). Detalhe em cada `docs/game-models/<archetype>-dataplane-contract.md`.

Não confundir com o **JWT de identidade** (emitido pela identity-service, prova "sou o jogador X" para o backend, vida longa): o connect token é de **uma entrada só**, curtíssimo, e prova autorização para uma partida específica ao *game server*.

---

## 8. Observabilidade como instrumento da tese

`slog` JSON (logs) + Prometheus (métricas) + Grafana (dashboards) + OpenTelemetry (traces). A mesma stack roda local e em prod, e alimenta a comparação Go-vs-AWS: custo $/mês, latência p50/p95/p99, throughput, uso de recurso. (ADR-0006)

---

## 9. Índice de decisões

Todas as decisões e seus trade-offs estão em `docs/adr/README.md`. Antes de implementar algo arquitetural, confira se há ADR aplicável; se conflitar com um ADR aceito, avise antes de prosseguir.

---

## Convenção de diagramas

Todo diagrama neste repositório é feito em **Mermaid** (GitHub renderiza nativo). ASCII art não é usado para diagramas. Listagens de árvore de arquivos/pastas permanecem em bloco de código texto, pois Mermaid não as representa bem.
