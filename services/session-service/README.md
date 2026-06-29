# session-service 🚧

≈ **GameLift (orquestração de frota/sessão)** — **capstone** do control plane (ADR-0010). A partir de um match, aloca um game server, emite o **connect token (Ed25519)** que o Unity Dedicated Server valida, e recebe o resultado da partida (com fan-out para os serviços de dados).

> **Status: em desenvolvimento.** Spec e plan aprovados (`specs/001-session-core/`); implementação em andamento.

- **Protocolo:** gRPC (unary no MVP) + HTTP (JWKS do connect token, métricas)
- **Storage:** memória (sessões ativas; pool de game servers mock)
- **Dependências:** matchmaking (recebe match), identity (control plane interno)

## Contrato previsto

```
service Session {
  rpc CreateSession(CreateSessionRequest) returns (CreateSessionResponse); // aloca server + connect tokens
  rpc ReportResult(ReportResultRequest) returns (ReportResultResponse);    // resultado + fan-out
}
```

No MVP: allocator **mock** (Agones real é deploy, ADR-0011/0012), fan-out via **stub** que loga (o fan-out HTTP real exige endpoints server-side + service-auth — próxima feature).

## Specs

[`specs/`](specs/) — `001-session-core`
