# Spec: 007-admin-session-control

**Serviço:** session-service
**Status:** Done
**Data:** 2026-07-04
**Parte de:** Rooms Console (Onda 1) — pares: `admin-bff/008-rooms`, `admin-ui/009-rooms-console`

## Contexto

O admin do session-service hoje é **read-only e por-jogo** (`GET /admin/games/{gameId}/sessions`, spec 005). O console administrativo precisa de duas capacidades novas para gerenciar salas:

1. **Visão global** — listar as sessões de **todos os jogos** de uma vez (o board "Live Rooms"), não só por jogo.
2. **Encerrar sala travada** — um verbo de gestão. Hoje uma sessão só fecha por `ReportResult` (gRPC do game server) ou pelo TTL sweeper; se o game server morre sem reportar e o TTL ainda não venceu, a sala fica "zumbi" e não há como um operador encerrá-la.

Fechamento já é unificado internamente (`closeSession` marca CLOSED, libera o server via allocator, entra no histórico) — usado por `ReportResult` (`result`) e pelo sweeper (`ttl`). Esta spec adiciona o motivo **`admin`** e o expõe.

## Comportamento

- WHEN `GET /admin/sessions[?state=open|closed]` -> THE SYSTEM SHALL retornar as sessões de **todos os jogos** (ativas e fechadas conforme o filtro), cada uma com o `gameId`, no mesmo shape do endpoint por-jogo (spec 005). Sem filtro: ativas + fechadas.
- WHEN `POST /admin/sessions/{matchId}/close` -> THE SYSTEM SHALL encerrar a sessão ativa (CLOSED, motivo **`admin`**), liberar o game server (allocator) e devolver a sessão encerrada.
- IF não existir sessão **ativa** para o `matchId` -> `404 NotFound` (já fechada ou inexistente; idempotente do ponto de vista do operador).
- THE SYSTEM SHALL contabilizar o fechamento em `session_closed_total{reason="admin"}` (métrica existente por motivo).
- O endpoint fica sob `/admin` (internal key na borda, ADR-0018), como os demais.

## Contratos de Interface

- **Domain:** nova `CloseReason` `CloseAdmin = "admin"`.
- **Service:**
  - `AllSessions() (active, closed []domain.Session)` — irmão global do `SessionsByGame`; ordena por `OpenedAt`/`ClosedAt` desc.
  - `CloseByAdmin(ctx, matchID) (*domain.Session, error)` — reusa `closeSession(ctx, matchID, CloseAdmin)`; `ErrSessionNotFound` se não estiver ativa.
- **Transport (admin HTTP):** `GET /admin/sessions` e `POST /admin/sessions/{matchId}/close` no `AdminHandler`. Reusa o `sessionDTO` da spec 005.

## Casos de Teste Obrigatórios

- [ ] `AllSessions` devolve sessões de jogos diferentes juntas (ativas + fechadas)
- [ ] `CloseByAdmin` de sessão ativa -> CLOSED/`admin`, server liberado (allocator.Release), some das ativas
- [ ] `CloseByAdmin` de match inexistente/já fechado -> `ErrSessionNotFound`
- [ ] `GET /admin/sessions?state=open` -> só ativas; sem filtro -> ativas + fechadas
- [ ] `POST /admin/sessions/{m}/close` inexistente -> 404

## Fora do escopo

- **Reportar resultado manual** no close (decisão de escopo da Onda 1: só encerrar+liberar; não dispara fan-out).
- Paginação do board global (histórico é bounded em memória; volume de dev).
- Persistência das sessões (segue in-memory; ver ADR-0020 gatilhos de outbox).
