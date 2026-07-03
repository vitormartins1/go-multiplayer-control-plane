# Tasks: 004-agones-allocator

**Plan:** [plan.md](plan.md)
**Data:** 2026-07-03

## Domain

- [ ] `Session` + `State/OpenedAt/ClosedAt/CloseReason`; `Allocate(ctx, matchID, gameID)`; static/testes ajustados

## Allocator Agones

- [ ] `allocator/agones.go`: `GameServerAllocation` via dynamic client (metadata match_id/game_id), `Release` = delete GameServer (NotFound ok)
- [ ] Decorator `DEV_PUBLIC_HOST` (host reescrito, porta mantida)
- [ ] Testes com fake dynamic client (Allocated, UnAllocated→ErrNoServer, Release idempotente)

## Service

- [ ] `closeSession(matchID, reason)` unificado (`ReportResult` → `result`)
- [ ] Sweeper TTL (`SESSION_TTL`, clock injetável, shutdown limpo) + testes
- [ ] Métrica `sessions_closed_total{reason}`

## Wiring / compose

- [ ] Envs + `ALLOCATOR_MODE` no main (default static)
- [ ] Compose: kubeconfig do k3d montado + profile/override `agones` (default segue static)

## Verificação integrada (com o data plane, spec 005-agones)

- [ ] CreateSession → GameServer Allocated com annotations; endereço real na resposta
- [ ] Fleet esgotada → ResourceExhausted
- [ ] ReportResult (grpcurl) → CLOSED(result) + GameServer deletado
- [ ] TTL vencido → CLOSED(ttl) + GameServer deletado
- [ ] Chave da annotation validada par a par com o Unity (fixar nos dois specs)

## Fechamento

- [ ] CI verde (modo static sem cluster) · README do serviço atualizado · commits `feat(session)/...`, merge --no-ff
