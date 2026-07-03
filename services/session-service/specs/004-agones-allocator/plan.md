# Plan: 004-agones-allocator

**Spec:** [spec.md](spec.md)
**Data:** 2026-07-03

Ordem: domain (estado/TTL) → allocator Agones → wiring/config → compose → integração com o k3d do data plane.

## 1. Domain: estado da sessão + Port

- `domain/session.go`: `Session` + `State (open|closed)`, `OpenedAt`, `ClosedAt`, `CloseReason (result|ttl)`.
- `ServerAllocator.Allocate(ctx, matchID, gameID string) (Server, error)` — o static ignora os args (mudança mecânica nos testes).
- Novo Port? Não — Release já existe. Sweeper é serviço, não Port.

## 2. Adapter `allocator/agones.go`

- **Decisão (recomendada): k8s API via client-go** — criar `GameServerAllocation` (agones.dev/v1) no namespace configurado:
  ```yaml
  spec:
    selectors: [{ matchLabels: { "agones.dev/fleet": $AGONES_FLEET } }]
    metadata:
      annotations: { match_id: ..., game_id: ... }   # ← chave exata: validar com o data plane
  ```
  Status da resposta: `state` (`Allocated`/`UnAllocated`), `gameServerName`, `address`, `ports[0].port`.
- Client: `k8s.io/client-go` + dynamic client (evita depender do types do Agones; ou `agones.dev/agones/pkg/client` se o go.mod aceitar bem — avaliar peso do módulo).
- `Release(serverID)`: delete do GameServer por nome; `IsNotFound` → ok (Unity já fez Shutdown).
- Kubeconfig: `KUBECONFIG` (dev/compose com arquivo montado) ou in-cluster (futuro k3s).
- Testes: fake do dynamic client (client-go `fake`), sem cluster.

## 3. Service: TTL sweeper + fechamento unificado

- `service/`: extrair um `closeSession(matchID, reason)` usado por `ReportResult` (reason `result`) e pelo **sweeper** (goroutine com ticker; sessões `OpenedAt + SESSION_TTL < now` → reason `ttl`). Lifecycle da goroutine no main (context + shutdown limpo).
- Métricas: counter `sessions_closed_total{reason}` (padrão OBSERVABILITY.md).

## 4. Config / wiring (`cmd/server/main.go`)

- Envs: `ALLOCATOR_MODE` (default `static`), `AGONES_NAMESPACE`, `AGONES_FLEET`, `DEV_PUBLIC_HOST`, `SESSION_TTL` (default `30m`).
- `DEV_PUBLIC_HOST`: decorator no allocator (reescreve host do `Server.Address`) — não vaza para o domain.

## 5. Compose / integração local

- `docker/docker-compose.yml`: montar kubeconfig do k3d no session-service (`~/.k3d/kubeconfig-packet-arena.yaml:/kubeconfig`, `KUBECONFIG=/kubeconfig`) + `ALLOCATOR_MODE=agones` via override/profile — **default do compose continua `static`** (quem não subiu o k3d não quebra).
- Atenção: de dentro do compose, o API server do k3d é alcançável via `host.docker.internal:<porta do k3d>` — gerar kubeconfig com `k3d kubeconfig get` e ajustar o server host (documentar no README do spec par, `tools/k3d/`).

## Riscos / notas

- **go.mod**: client-go é pesado mas padrão; módulo `agones.dev/agones` inteiro pode trazer muito — preferir dynamic client + GVR de `gameserverallocations`/`gameservers`.
- **Annotation**: metadata de allocation aplica no GameServer sem prefixo `agones.dev/sdk-`; o SDK `GetGameServer` do lado Unity lê `objectMeta.annotations` cru — validar par a par no primeiro teste integrado (risco cruzado com o data plane).
- **Relógio do TTL**: usar `time.Now` injetável (clock) para o sweeper ser testável.
- `ErrNoServer` já mapeia para `ResourceExhausted` no transport — sem mudança de contrato gRPC.
