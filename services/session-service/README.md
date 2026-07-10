# session-service 🚧

≈ **GameLift (orquestração de frota/sessão)** — **capstone** do control plane (ADR-0010). A partir de um match, aloca um game server, emite o **connect token (Ed25519)** que o Unity Dedicated Server valida, e recebe o resultado da partida (com fan-out para os serviços de dados).

> **Status: em desenvolvimento.** Sessões com ciclo de vida (OPEN/CLOSED + TTL) e alocação real via **Agones** (spec `004-agones-allocator`).

- **Protocolo:** gRPC (unary no MVP) + HTTP (JWKS do connect token, métricas)
- **Storage:** memória (sessões ativas)
- **Dependências:** matchmaking (recebe match), identity (control plane interno); cluster Agones no modo `agones`

## Contrato previsto

```
service Session {
  rpc CreateSession(CreateSessionRequest) returns (CreateSessionResponse); // aloca server + connect tokens
  rpc ReportResult(ReportResultRequest) returns (ReportResultResponse);    // resultado + fan-out
}
```

## Allocator (spec 004)

- `ALLOCATOR_MODE=static` (default): pool mock em memória (`SERVER_POOL`) — compose e testes funcionam sem cluster.
- `ALLOCATOR_MODE=agones`: cria `GameServerAllocation` via k8s API (dynamic client) com annotations `match_id`/`game_id` (chave **sem prefixo** — validado empiricamente; o Unity lê `objectMeta.annotations` cru e rearma o gate). Envs: `KUBECONFIG` (ou in-cluster), `AGONES_NAMESPACE` (default `default`). A **Fleet não é mais fixa por env var** (ADR-0026, spec 022): cada sessão resolve a Fleet do perfil do jogo (game-registry, `PUT /games/{id}/fleet` — spec `game-registry/012`). **Runbook:** antes de habilitar `ALLOCATOR_MODE=agones` para um jogo, (1) crie a Fleet correspondente no cluster (k3d/Pi5) e (2) cadastre o nome no jogo via `PUT /games/{id}/fleet`; sem isso, a 1ª `CreateSession`/`OpenWorld` do jogo falha com `FailedPrecondition` ("fleet not configured") em vez de arriscar alocar do binário errado.
- `DEV_PUBLIC_HOST` (dev): reescreve o host do endereço devolvido mantendo a porta dinâmica (IP interno do node k3d → `127.0.0.1`; portas mapeadas pelo k3d).
- Sessões fecham por `ReportResult` (reason `result`) ou pelo sweeper de TTL (`SESSION_TTL`, default `30m`, reason `ttl`); fechar = `Release` (no Agones, deleta o GameServer — a Fleet repõe).
- Compose com k3d: `docker compose -f docker-compose.yml -f docker-compose.agones.yml up -d session-service` (ver `infra/docker/docker-compose.agones.yml`).

Métricas: `session_active_sessions` (gauge), `sessions_closed_total{reason}` (counter).

## Specs

[`specs/`](specs/) — `001-session-core` · `002-result-fanout` · `003-leaderboard-fanout` · `004-agones-allocator` · `005-admin-sessions`
