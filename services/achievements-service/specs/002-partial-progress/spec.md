# Spec: 002-partial-progress

**Serviço:** achievements-service
**Status:** Approved
**Data:** 2026-06-28

## Contexto

Estende a 001-achievement-core com **progresso parcial**: conquistas que exigem N ocorrências de um evento (ex. "vença 10 partidas"), rastreando o progresso do jogador (7/10) até o desbloqueio. **Generaliza** o desbloqueio direto da 001 — que passa a ser o caso particular `target = 1`.

Decisão de escopo (ver "Fora do escopo"): **cada evento recebido conta como uma ocorrência** (sem deduplicação). Idempotency-key para reenvios é evolução futura.

## Comportamento

### Catálogo com meta
- THE SYSTEM SHALL associar a cada conquista uma meta `target` inteira ≥ 1; `target = 1` é desbloqueio direto (comportamento da 001).
- WHEN GET /achievements (autenticado) -> inclui o `target` de cada conquista.

### Progresso por evento
- WHEN POST /events (autenticado) com `type` que casa uma conquista **ainda não desbloqueada** -> incrementa o progresso do jogador naquela conquista em 1.
- IF o progresso atingir o `target` -> desbloqueia a conquista e a inclui na lista de **recém-desbloqueadas** da resposta.
- IF a conquista já estiver desbloqueada -> não incrementa e não a inclui nas recém-desbloqueadas (idempotência do desbloqueio).
- IF nenhuma conquista casar o `type` -> resposta com lista vazia.
- THE SYSTEM SHALL contar **cada evento** recebido como uma ocorrência (sem deduplicação de reenvio no MVP).

### Listar progresso do jogador
- WHEN GET /me/achievements (autenticado) -> cada conquista do catálogo com `progress` (0..`target`), `target`, `unlocked` e `unlockedAt` (quando desbloqueada).
- THE SYSTEM SHALL limitar o `progress` exibido ao `target` (nunca exibir além da meta).

## Contratos de Interface (mudanças sobre a 001)

### AchievementRepository (Port de saída)
- `RecordProgress(ctx, playerID, achievementID string, target int, at time.Time) (justUnlocked bool, err error)` — incrementa o progresso de forma atômica; desbloqueia (grava `unlockedAt`) ao atingir `target`; **no-op** se já desbloqueada. Substitui `Unlock`.
- `ListPlayerProgress(ctx, playerID string) ([]PlayerProgress, error)` — progresso + estado de desbloqueio por conquista. Substitui `ListPlayerUnlocks`.
- `ListCatalog`, `FindByEventType` — inalterados (passam a trazer `Target`).

### Domínio
- `Achievement` ganha o campo `Target int`.
- Novo `PlayerProgress{ AchievementID string; Progress int; UnlockedAt time.Time (zero = em progresso) }`.

## Modelo de Dados (mudanças)

**achievements** (catálogo)

| Campo | Mudança |
|---|---|
| target | **novo:** INT NOT NULL DEFAULT 1 (meta de ocorrências) |

**player_achievements**

| Campo | Mudança |
|---|---|
| progress | **novo:** INT NOT NULL DEFAULT 0 (ocorrências acumuladas) |
| unlocked_at | passa a aceitar **NULL** (NULL = em progresso; preenchido ao desbloquear) |
| (PK) | `(player_id, achievement_id)` mantido — uma linha por conquista em progresso/desbloqueada |

## Regras de Validação

| Item | Regra | Erro |
|---|---|---|
| Token | (igual 001) | ErrUnauthorized |
| Event.Type | obrigatório, não vazio | ErrInvalidEvent |
| Achievement.target | inteiro ≥ 1 (garantido no catálogo/seed) | — |

## Casos de Teste Obrigatórios

- [ ] Evento casa conquista com `target>1` -> incrementa o progresso **sem** desbloquear (progress < target)
- [ ] Eventos sucessivos atingem o `target` -> desbloqueia; resposta lista a recém-desbloqueada
- [ ] Evento após desbloqueada -> `progress` e `unlockedAt` inalterados; não retorna como nova
- [ ] Conquista `target=1` -> desbloqueia no primeiro evento (compatível com a 001)
- [ ] GET /me/achievements reflete `progress`, `target`, `unlocked` e `unlockedAt`; progress nunca passa de target
- [ ] Evento sem match -> resposta vazia, nenhum progresso alterado
- [ ] Progresso isolado entre dois jogadores

## Fora do escopo

- **Deduplicação de evento** (idempotency-key / event_id) — reenvios contam; dedup é evolução futura
- **Decremento / reset** de progresso
- **Conquistas secretas e recompensas** — backlog
- **CRUD de admin do catálogo** — backlog (catálogo é seed)
