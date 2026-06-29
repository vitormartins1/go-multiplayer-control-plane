# Spec: 005-server-events

**Serviço:** achievements-service
**Status:** Approved
**Data:** 2026-06-29

## Contexto

Habilita o **fan-out do resultado** (ADR-0010/0017/0018): num modelo server-authoritative, o **game server / session** reporta eventos de gameplay **em nome do jogador**, sem o JWT dele. Novo endpoint **admin interno** que registra um evento para um `playerID` num jogo. É só **transport** — reusa o `RecordEvent` (avaliação/idempotência de 001/002/004). Protegido pela internal key (ADR-0018, já ligada nos `/admin`).

## Comportamento

- WHEN `POST /admin/games/{gameId}/players/{playerID}/events {type}` -> avalia o evento no jogo `{gameId}` para `{playerID}` e retorna `{ unlocked: [...] }` (conquistas recém-desbloqueadas).
- IF `type` vazio -> `400` (**invalid_event**).
- THE SYSTEM SHALL resolver `gameId` e `playerID` do **path** (servidor reporta; não há JWT do jogador aqui).
- Auth: `/admin/...` exige a internal key serviço-a-serviço (ADR-0018).

## Contratos de Interface

| Método | Rota | Retorno |
|---|---|---|
| POST | `/admin/games/{gameId}/players/{playerID}/events` | `{ "unlocked": [{id,name,description,target}] }` |

Reusa `AchievementService.RecordEvent(ctx, gameID, playerID, Event)` — sem mudança em domain/service/repository.

## Casos de Teste Obrigatórios

- [ ] `POST .../events {type}` -> avalia para o `playerID`/`gameId` do path; retorna `unlocked`
- [ ] `type` vazio -> `400`
- [ ] o endpoint não usa JWT (playerID do path); fica sob `/admin` (internal key valida na borda)

## Fora do escopo

- **Caller do session** (fan-out de `ReportResult`) — feature do session (peça 4)
- **game_id no match** (matchmaking→session) — peça 3
- Mudanças em domain/service/repository — reusa o existente
