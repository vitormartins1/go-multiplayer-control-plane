# Spec: 001-achievement-core

**Serviço:** achievements-service
**Status:** Approved
**Data:** 2026-06-28

## Contexto

Núcleo do achievements-service (≈ GameKit Achievements). Cobre o **catálogo de conquistas** (definido por seed), o **desbloqueio por evento de gameplay** (evento → avalia critério → desbloqueia, de forma idempotente) e a **listagem das conquistas do jogador** (desbloqueadas + as ainda bloqueadas). É o primeiro serviço a **validar o JWT emitido pelo identity** (ADR-0009): não tem login próprio — confia no token do identity, validando-o via JWKS.

Decisões de escopo desta feature (ver "Fora do escopo" para o que fica de fora):
- **Ingestão por HTTP síncrono** (`POST /events`). O core é hexagonal e isola o transporte: trocar a ingestão por um consumer de fila no futuro não toca o domínio. A decisão de tornar o **fan-out de resultado** (session → leaderboard/achievements/analytics) event-driven pertence ao **session-service** e será um ADR quando ele for desenhado (ver `docs/FEATURE-MAP.md`).
- **Critério de desbloqueio direto** (threshold = 1): um evento de um tipo desbloqueia a conquista associada. Progresso parcial (contadores 7/10) é a feature 002.
- **Catálogo por seed** (migration); a API só lê o catálogo, sem CRUD de admin.

## Comportamento

### Autenticação (todos os endpoints)
- THE SYSTEM SHALL exigir um `Authorization: Bearer <jwt>` válido em todos os endpoints de negócio.
- IF o token estiver ausente, malformado, expirado ou com assinatura inválida -> retorna 401 com **ErrUnauthorized**.
- THE SYSTEM SHALL validar a assinatura do JWT com a chave pública do identity, obtida do **JWKS** (`/.well-known/jwks.json`), casando pelo `kid` do header.
- THE SYSTEM SHALL **cachear o JWKS** e revalidá-lo periodicamente — nunca buscar o JWKS a cada request.
- THE SYSTEM SHALL identificar o jogador pelo `sub` do token (o `playerID`), e **nunca** confiar em um id de jogador vindo do corpo da requisição.

### Ingerir evento de gameplay
- WHEN POST /events (autenticado) com `{ type }` válido -> avalia o catálogo e, para cada conquista cujo critério casa com `type`, desbloqueia-a para o jogador; retorna 200 com a lista das conquistas **recém-desbloqueadas** nesta chamada.
- IF `type` estiver ausente ou vazio -> retorna 400 com **ErrInvalidEvent**.
- IF o evento casar uma conquista que o jogador **já desbloqueou** -> não desbloqueia de novo e **não** a inclui na lista de recém-desbloqueadas (idempotência).
- IF o evento não casar nenhuma conquista -> retorna 200 com lista vazia.
- THE SYSTEM SHALL ser **idempotente**: processar o mesmo desbloqueio mais de uma vez não cria duplicata nem altera o `unlockedAt` original.

### Listar catálogo
- WHEN GET /achievements (autenticado) -> retorna 200 com o catálogo de conquistas (`id`, `name`, `description`).

### Listar conquistas do jogador
- WHEN GET /me/achievements (autenticado) -> retorna 200 com **todas** as conquistas do catálogo anotadas com o estado do jogador: `unlocked` (bool) e `unlockedAt` (quando desbloqueada).
- THE SYSTEM SHALL refletir apenas os desbloqueios do jogador identificado pelo token.

## Contratos de Interface

### AchievementService (Port de entrada)
- ListCatalog(ctx) -> ([]Achievement, error)
- ListForPlayer(ctx, playerID) -> ([]PlayerAchievement, error)   // catálogo anotado com o estado do jogador
- RecordEvent(ctx, playerID, event Event) -> (unlocked []Achievement, error)   // avalia e desbloqueia (idempotente)

### AchievementRepository (Port de saída)
- ListCatalog(ctx) -> ([]Achievement, error)
- FindByEventType(ctx, eventType) -> ([]Achievement, error)   // conquistas disparadas por um tipo de evento
- ListPlayerUnlocks(ctx, playerID) -> ([]Unlock, error)
- Unlock(ctx, playerID, achievementID, at) -> (created bool, error)   // idempotente: cria só se ainda não existir

### TokenVerifier (Port de saída, usada no transport)
- Verify(ctx, bearerToken) -> (playerID string, error)   // valida via JWKS do identity; retorna o `sub`

## Modelo de Dados

**Achievement** (catálogo — definido por seed)

| Campo | Tipo | Restrições |
|---|---|---|
| ID | string (slug) | identificador estável, ex. `first_win` |
| Name | string | nome exibível |
| Description | string | descrição exibível |
| EventType | string | tipo de evento que dispara o desbloqueio (critério MVP) |

**Unlock** (desbloqueio de um jogador)

| Campo | Tipo | Restrições |
|---|---|---|
| PlayerID | string (UUID) | id do jogador (`sub` do JWT) |
| AchievementID | string (slug) | FK para Achievement |
| UnlockedAt | timestamp | preenchido no primeiro desbloqueio; imutável |
|  | **PK (PlayerID, AchievementID)** | unicidade garante idempotência |

**Event** (entrada; não necessariamente persistido no MVP)

| Campo | Tipo | Origem |
|---|---|---|
| Type | string | corpo da requisição |
| PlayerID | string | **do JWT (`sub`)**, nunca do corpo |

## Regras de Validação

| Item | Regra | Erro |
|---|---|---|
| Token | presente, bem-formado, não expirado, assinatura válida (JWKS) | ErrUnauthorized |
| Event.Type | obrigatório, não vazio | ErrInvalidEvent |
| Desbloqueio | único por (jogador, conquista) | (idempotente, sem erro) |

## Casos de Teste Obrigatórios

- [ ] Catálogo carregado por seed; GET /achievements retorna as definições
- [ ] POST /events com `type` que casa uma conquista -> desbloqueia; resposta lista a nova conquista
- [ ] POST /events repetido com o mesmo `type` -> idempotente; não desbloqueia de novo; resposta com lista vazia; `unlockedAt` inalterado
- [ ] POST /events com `type` que não casa nada -> 200 com lista vazia
- [ ] POST /events sem `type` -> 400 ErrInvalidEvent
- [ ] GET /me/achievements reflete o estado do jogador (unlocked + unlockedAt) e inclui as ainda bloqueadas
- [ ] Requisição sem Bearer -> 401 ErrUnauthorized
- [ ] Requisição com JWT expirado ou assinatura inválida -> 401 ErrUnauthorized
- [ ] O `playerID` usado vem do `sub` do JWT — um id no corpo do evento é ignorado
- [ ] Dois jogadores diferentes têm desbloqueios independentes

## Fora do escopo

- **Progresso parcial / contadores** (ex. 7/10) — feature 002
- **Conquistas secretas e recompensas associadas** — backlog
- **CRUD de admin do catálogo** — backlog (catálogo é seed no MVP)
- **Fan-out event-driven (SQS) do resultado de partida** — decisão do session-service (ADR futuro); a ingestão aqui é HTTP, com o core desacoplado para permitir a troca
- **Trust boundary do emissor:** no MVP o próprio jogador autenticado posta o evento (mock até o session existir). Numa fase futura, o emissor confiável do resultado será o session/game server — o cliente não deve poder forjar "ganhei". Não tratado aqui.
- **Rotação/refresh do cache de JWKS sob falha do identity** além do refresh periódico simples
