# ADR-0011: Unity Netcode for GameObjects + Dedicated Server, sem serviços pagos; Agones/EKS para escala

**Status:** Accepted
**Data:** 2026-06-27
**Serviço(s) afetado(s):** data plane (Unity — repo separado), infra de game server

---

## Contexto

Definido o data plane no Unity Dedicated Server (ADR-0010), resta escolher o stack de netcode e como hospedar/escalar os game servers sem depender de serviços pagos (Unity Gaming Services ou GameLift), coerente com a tese (ADR-0008).

## Alternativas Consideradas

### Opção A: Netcode for GameObjects + Unity Transport + Dedicated Server
- **Prós:** oficial, gratuito/open-source; fluxo tradicional da Unity (MonoBehaviour, animações, plugins); melhor ponto de partida prático; reaproveita projetos existentes.
- **Contras:** menos performático/data-oriented que ECS.

### Opção B: Netcode for Entities (ECS/DOTS)
- **Prós:** mais moderno e performático; server-authoritative orientado a dados; ghosts/snapshots/lag compensation.
- **Contras:** curva alta; exige modelar em ECS desde cedo; atrasaria aprender Go+AWS+Unity ao mesmo tempo.

### Opção C: Unity Gaming Services / GameLift gerenciados
- **Prós:** prontos.
- **Contras:** pagos; vendor lock-in; contrariam ADR-0008.

## Decisão

Data plane usa **Netcode for GameObjects + Unity Transport + Dedicated Server build**. Os serviços pagos da Unity (Relay, Lobby, Matchmaker, Multiplay) são **substituídos por serviços Go** + conexão direta por IP/porta com connect token. Escala de game server evolui em níveis: **EC2/VPS com Docker → ECS-on-EC2 → Agones/EKS** (orquestrador open-source de game servers em Kubernetes, equivalente livre ao GameLift/Multiplay). **Nunca Fargate** para o game server realtime (UDP, portas dinâmicas, ciclo de sessão). **Netcode for Entities/ECS** fica como laboratório avançado futuro.

## Consequências

### Positivas
- Gratuito/open-source; controle de custo; alinhado à tese; caminho de escala claro; dá ao EKS uma aplicação concreta (Agones).

### Negativas / Trade-offs
- Curva de aprendizado da Unity multiplayer; operar Agones/EKS quando escalar; Netcode for GameObjects menos performático que ECS.

### Neutras / Implicações
- Unity Client e Dedicated Server são **o mesmo projeto Unity** (2 build targets: client com gráficos + server headless), num repo separado do backend Go — ~2 repos no total. Contrato com o backend via `.proto`.
- O Go **não** participa do loop realtime por tick; só do control plane e de chamadas fora do caminho crítico (token, heartbeat, resultado). Ver ADR-0010.
- A infra do Dedicated Server na AWS (deploy, fleet, alocação por sessão) e um template reutilizável para outros jogos são trabalho futuro (backlog).

## Referências
- Unity Netcode for GameObjects / Unity Transport — docs.unity3d.com
- Agones — agones.dev
