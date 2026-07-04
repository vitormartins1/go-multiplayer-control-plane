# Plan: 007-admin-session-control

**Spec:** specs/007-admin-session-control/spec.md
**Data:** 2026-07-04

## Abordagem técnica

Reusa o fechamento unificado (`closeSession`) e o `AdminHandler` da spec 005. Adiciona o motivo `admin`, dois métodos no service (listagem global + close por admin) e duas rotas no admin HTTP. O `sessionDTO` ganha `gameId` (o board global precisa saber de qual jogo é cada sala).

## Arquivos

- `internal/domain/session.go` — nova `CloseReason` `CloseAdmin = "admin"`.
- `internal/service/session_service.go`:
  - `AllSessions() (active, closed []domain.Session)` — varre `s.sessions` (ativas) + `s.history` (fechadas) sob o mutex; ordena desc por `OpenedAt`/`ClosedAt` (espelha `SessionsByGame`).
  - `CloseByAdmin(ctx, matchID) (*domain.Session, error)` — `closeSession(ctx, matchID, CloseAdmin)`; devolve `ErrSessionNotFound` se não ativa.
- `internal/transport/http_handler.go`:
  - `SessionDirectory` ganha `AllSessions()` + `CloseByAdmin(ctx, matchID)`.
  - `sessionDTO` ganha `GameId`.
  - `GET /admin/sessions` (listAll, mesmo filtro `state`), `POST /admin/sessions/{matchId}/close` (close → 200 DTO | 404).
- `internal/transport/http_handler_test.go` — testes dos dois novos endpoints (httptest + fake directory).

## Decisões

- **Interface do consumidor** (`SessionDirectory`) cresce em vez de criar outra — o `*service.Service` já satisfaz; sem mudança no wire-up do `main.go`.
- **404 no close** de sessão não-ativa (idempotente do ponto de vista do operador; não é erro de servidor).
- **`gameId` no DTO** é aditivo — a aba Sessions por-jogo (005/007) ignora o campo novo, não quebra.
- `AllSessions` não pagina (histórico já é bounded em memória; volume de dev).

## Ordem

1. domain (`CloseAdmin`) + service (`AllSessions`, `CloseByAdmin`) + testes de service.
2. transport (rotas + `gameId` no DTO) + testes httptest.
3. `go test -race` + build.

## Riscos

- **Concorrência**: `AllSessions` lê `sessions`+`history` sob o mesmo mutex do `closeSession` (sem novo lock). Copiar slices antes de ordenar (não segurar o lock na ordenação).
- **DTO aditivo**: conferir que o cliente atual (BFF sessions/007) tolera o campo `gameId` extra (tolera — desserializa por nome).
