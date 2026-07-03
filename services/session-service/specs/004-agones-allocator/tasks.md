# Tasks: 004-agones-allocator

**Plan:** [plan.md](plan.md)
**Data:** 2026-07-03

## Domain

- [x] `Session` + `State/OpenedAt/ClosedAt/CloseReason`; `Allocate(ctx, matchID, gameID)`; static/testes ajustados

## Allocator Agones

- [x] `allocator/agones.go`: `GameServerAllocation` via dynamic client (metadata match_id/game_id), `Release` = delete GameServer (NotFound ok)
- [x] Decorator `DEV_PUBLIC_HOST` (host reescrito, porta mantida)
- [x] Testes com fake dynamic client (Allocated, UnAllocated→ErrNoServer, Release idempotente)

## Service

- [x] `closeSession(matchID, reason)` unificado (`ReportResult` → `result`)
- [x] Sweeper TTL (`SESSION_TTL`, clock injetável, shutdown limpo) + testes
- [x] Métrica `sessions_closed_total{reason}`

## Wiring / compose

- [x] Envs + `ALLOCATOR_MODE` no main (default static)
- [x] Compose: kubeconfig do k3d montado + profile/override `agones` (default segue static)

## Verificação integrada (com o data plane, spec 005-agones)

- [ ] CreateSession → GameServer Allocated com annotations; endereço real na resposta
- [ ] Fleet esgotada → ResourceExhausted
- [ ] ReportResult (grpcurl) → CLOSED(result) + GameServer deletado
- [ ] TTL vencido → CLOSED(ttl) + GameServer deletado
- [ ] Chave da annotation validada par a par com o Unity (lado k8s validado via kubectl: sem prefixo, specs fixados; falta o lado Unity via sidecar)

## Fechamento

- [ ] CI verde (modo static sem cluster) · README do serviço atualizado · commits `feat(session)/...`, merge --no-ff
