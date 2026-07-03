# Spec: 004-agones-allocator

**Serviço:** session-service
**Status:** Draft
**Data:** 2026-07-03

## Contexto

Troca o pool mock (`allocator/static.go`) pela **alocação real via Agones** (ADR-0011/0012): o Unity Dedicated Server (fase 4 do data plane) roda como GameServer numa Fleet em **k3d local** (depois k3s do Pi 5), e o `CreateSession` passa a reservar um GameServer de verdade. Spec par no data plane: `go-game-dataplane/docs/specs/005-agones`.

Também dá **ciclo de vida** à sessão: hoje ela só morre via `ReportResult` (que o Unity ainda não chama — fase 6). Entram **estado** (OPEN/CLOSED) e **TTL**, para sessões abandonadas fecharem sozinhas e o GameServer ser reciclado. Base para a superfície admin (spec `005-admin-sessions`).

## Comportamento

### Alocação via Agones

- WHEN `CreateSession{matchId, gameId, players[]}` → o allocator SHALL criar uma **`GameServerAllocation`** no cluster (k8s API, kubeconfig/in-cluster por config) com **metadata da partida** (`match_id`, `game_id`) aplicada ao GameServer alocado — é assim que o server Unity descobre seu match e rearma o gate.
- A `Session` SHALL registrar `ServerID` = nome do GameServer e `ServerAddress` = `status.address:port` da alocação.
- IF a alocação retornar `UnAllocated` (fleet sem `Ready`) → **`ErrNoServer`** (mesmo contrato de hoje: `ResourceExhausted` no gRPC).
- THE SYSTEM SHALL manter o **modo estático** (`ALLOCATOR_MODE=static|agones`, default `static`): compose e testes continuam funcionando sem cluster.

### Endereço público (dev)

- IF `DEV_PUBLIC_HOST` estiver setado (ex.: `127.0.0.1`) → o endereço devolvido nos tokens/`server_address` SHALL trocar o host pelo valor da env, **mantendo a porta dinâmica** (o range do Agones está mapeado do k3d para o host). Sem a env, endereço cru do Agones (comportamento de produção).

### Ciclo de vida e TTL

- A sessão SHALL ganhar estado **OPEN** (com `opened_at`) na criação e **CLOSED** (com `closed_at` + motivo) ao fechar.
- Motivos de fechamento: **`result`** (`ReportResult` — comportamento atual, continua liberando o server) e **`ttl`** (novo: sweeper fecha sessões abertas há mais que `SESSION_TTL`, default 30m).
- WHEN uma sessão fechar (qualquer motivo) → o allocator SHALL fazer **`Release(serverID)`**; no modo Agones, Release = **deletar o GameServer** (idempotente; se o Unity já fez `Shutdown()`, o objeto pode nem existir — não é erro). A Fleet repõe.

## Contratos de Interface

- `ServerAllocator` (Port, inalterada): `Allocate(ctx) (Server, error)` ganha os metadados da partida — **evoluir a assinatura** para `Allocate(ctx, matchID, gameID string)` (o static ignora).
- `Session` (domain): + `State`, `OpenedAt`, `ClosedAt`, `CloseReason`.
- Config: `ALLOCATOR_MODE`, `KUBECONFIG`/in-cluster, `AGONES_NAMESPACE` (default `default`), `AGONES_FLEET` (label selector), `DEV_PUBLIC_HOST`, `SESSION_TTL`.

## Casos de Teste / Aceite

- [ ] Modo `agones`: `CreateSession` → GameServer sai de `Ready` para `Allocated` com annotations `match_id`/`game_id`; resposta traz endereço:porta reais
- [ ] Fleet esgotada → `ResourceExhausted` (`ErrNoServer`)
- [ ] `DEV_PUBLIC_HOST=127.0.0.1` → `server_address` devolvido `127.0.0.1:<porta dinâmica>`
- [ ] `ReportResult` → sessão CLOSED(`result`) + GameServer deletado
- [ ] Sessão aberta > TTL → sweeper fecha CLOSED(`ttl`) + GameServer deletado
- [ ] Modo `static` (default): tudo como hoje (testes existentes passam sem cluster)
- [ ] Unit tests do adapter Agones com fake do client k8s (sem cluster no CI)

## Fora do escopo

- Allocator-service gRPC/mTLS do Agones (multi-cluster; local basta a k8s API — nota em Decisões)
- Persistência de sessões (segue memória; histórico bounded é da spec 005)
- FleetAutoscaler, EKS efêmero, homelab Pi (fases próprias)
- Auth serviço-a-serviço (ADR-0018, pendência conhecida)

## Decisões pendentes

- **k8s API × allocator gRPC**: recomendação k8s API (`GameServerAllocation` via client-go) — sem mTLS/cert bootstrap local; allocator gRPC volta quando houver multi-cluster (EKS). Confirmar no plan.
- Chave exata das annotations no GameServer (metadata da allocation × prefixo `agones.dev/sdk-`) — validar empiricamente com o data plane e fixar nos dois specs.
