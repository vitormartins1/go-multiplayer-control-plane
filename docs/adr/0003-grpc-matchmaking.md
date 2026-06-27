# ADR-0003: gRPC com streaming para matchmaking e controle de sessão

**Status:** Accepted
**Data:** 2026-06-27
**Serviço(s) afetado(s):** matchmaking-service, session-service

---

## Contexto

O matchmaking precisa notificar o cliente sobre o status do ticket (em fila, match encontrado, partida pronta) de forma contínua, e entregar o endpoint/token de conexão quando a partida fecha. Os serviços de CRUD (estilo GameKit) têm necessidade diferente: request/response simples.

## Alternativas Consideradas

### Opção A: gRPC com streaming (matchmaking) + REST (CRUD)
- **Prós:** contratos fortes via protobuf; streaming bidirecional eficiente para status de fila; mesmo IDL serve cliente Unity e serviços internos.
- **Contras:** curva de aprendizado de gRPC/protobuf; tooling adicional (codegen).

### Opção B: REST com polling
- **Prós:** simples.
- **Contras:** polling é ineficiente para status contínuo; sem contrato forte.

### Opção C: WebSocket
- **Prós:** streaming sobre HTTP.
- **Contras:** sem contrato tipado; reinventa o que o gRPC streaming já dá.

## Decisão

Usamos **gRPC com streaming** no matchmaking-service (e no controle de sessão em tempo real), e **REST** nos serviços de CRUD (identity, achievements, player-data, cloud-save). Protobuf como IDL. Atenção: gRPC é para *controle*; a sincronização realtime do gameplay roda no Unity Dedicated Server (ver ADR-0010), não em gRPC frame-a-frame.

## Consequências

### Positivas
- Status de fila eficiente; contratos versionados; reuso de proto entre cliente e backend.

### Negativas / Trade-offs
- Aprendizado de protobuf/gRPC e codegen no pipeline.

### Neutras / Implicações
- Os `.proto` ficam locais ao serviço (e podem ser compartilhados com o repo Unity como contrato).

## Referências
- gRPC Go — grpc.io/docs/languages/go
- AWS GameLift FlexMatch (modelo de fluxo de ticket)
