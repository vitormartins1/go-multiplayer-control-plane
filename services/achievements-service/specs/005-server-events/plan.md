# Plan: 005-server-events
**Spec:** specs/005-server-events/spec.md
**Data:** 2026-06-29

## Abordagem técnica
Só transport (ADR-0002): nova rota admin que lê `gameId`/`playerID` do path e
chama `RecordEvent`. Sem domain/service/repository. Já protegido pela internal
key (o guard cobre `/admin`).

## Arquivos
- `internal/transport/http_handler.go` — rota `POST /admin/games/{gameId}/players/{playerID}/events`; handler `adminRecordEvent` (decodifica `{type}`; chama `svc.RecordEvent`; responde `{unlocked}`)
- `internal/transport/http_handler_test.go` — 200 (unlocked); type vazio -> 400; usa path (não JWT)
- `api/achievements.http` — exemplo do endpoint (com X-Internal-Key)

## Decisões
- Reusa `RecordEvent` (mesma avaliação/idempotência do `/events` de jogador); muda só a origem de gameId/playerID (path).
- Sem auth no handler: a internal key valida na borda (main guard) — o handler é control plane interno.

## Ordem
1. rota + handler + test
2. api/.http; `go test -race` + `go build`
3. validar no Docker (com X-Internal-Key)

## Riscos
- Garantir que a rota fica sob `/admin` (senão escaparia do guard da internal key).
