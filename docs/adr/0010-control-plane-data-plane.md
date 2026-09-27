# ADR-0010: Control plane em Go, data plane realtime no Unity Dedicated Server

**Status:** Accepted (o data plane deixou de ser exclusivo do Unity: ver [ADR-0033](0033-engine-and-store-agnostic.md))
**Data:** 2026-06-27
**Serviço(s) afetado(s):** matchmaking-service, session-service, (data plane: Unity — repo separado)

---

## Contexto

A parte realtime do gameplay (input dos jogadores, tick da partida, estado do mundo, física, colisão, snapshots, prediction/reconciliation, tolerância a perda de pacote em UDP) é um problema profundamente diferente dos serviços de backend. Surgiu a questão de fazer um servidor autoritativo de partida em Go versus usar a solução da Unity.

Observação central: o próprio **GameLift não simula gameplay** — ele orquestra frota + matchmaking e roda o *seu* binário de servidor. Logo, reproduzir o GameLift fielmente é orquestrar, não simular.

## Alternativas Consideradas

### Opção A: Control plane (Go) + data plane (Unity Dedicated Server)
- **Prós:** usa a melhor ferramenta por camada; de-risca (não reimplementa netcode em Go); fiel a como o GameLift funciona; foco do Go fica em backend (o objetivo de carreira).
- **Contras:** introduz Unity no sistema (repo separado); precisa de contrato/handshake (connect token) entre os planos.

### Opção B: Servidor autoritativo de gameplay em Go
- **Prós:** tudo em Go.
- **Contras:** reescrever netcode (prediction, interpolation, UDP) é R&D enorme; não é o que se faz em backend Go enterprise; alto risco de descarrilar o projeto.

### Opção C: P2P / client-hosted
- **Prós:** sem servidor de partida.
- **Contras:** trapaça, NAT, sem autoridade; inadequado.

## Decisão

Separamos **control plane** (Go, este repo: auth, matchmaking, sessão, registry de servidores, persistência, leaderboard, achievements, analytics, resultado) de **data plane** (Unity Dedicated Server, repo separado: simulação realtime). O Go orquestra a partida (matchmaking + session-service emitindo connect token); o Unity Dedicated Server hospeda a simulação e valida o token contra o backend. `matchmaking + session ≈ orquestração do GameLift`; o Unity Dedicated Server é o build que se subiria no GameLift.

## Consequências

### Positivas
- De-risca o projeto; foco do Go em backend; comparação com GameLift mais honesta.

### Negativas / Trade-offs
- Sistema passa a ter dois mundos (Go + Unity) e um contrato entre eles; cliente/servidor Unity em repo separado.

### Neutras / Implicações
- Connect token é assinado/validado contra a identity-service (ADR-0009).
- O `gameplay-realtime-service` em Go foi descartado; vira `session-service` (control plane).

## Referências
- ADR-0011 (stack Unity e escala de game server)
- AWS GameLift (orquestração de frota + matchmaking)
