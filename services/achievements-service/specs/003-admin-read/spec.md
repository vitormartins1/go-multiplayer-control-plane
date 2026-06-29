# Spec: 003-admin-read

**Serviço:** achievements-service
**Status:** Approved
**Data:** 2026-06-29

## Contexto

O `admin-bff` (ADR-0016) precisa ler os achievements de um **jogador arbitrário** para o painel de administração. Os endpoints atuais do achievements são **auth-scoped ao `sub` do JWT** (`GET /me/achievements`) — servem o próprio jogador, não um admin consultando outro. Esta feature expõe um endpoint **admin read-only** que recebe o `playerID` no path.

É **control plane interno** — **sem auth** no MVP (consistente com session/matchmaking e com a decisão do `admin-bff`; auth real é evolução, provável ADR futuro). Reaproveita a lógica existente: `AchievementService.ListForPlayer` e `ListCatalog` já existem (001/002) — esta feature é **só transport** (rotas novas), sem mudança em service/domain/repository.

## Comportamento

### Achievements de um jogador (admin)
- WHEN `GET /admin/players/{playerID}/achievements` -> retorna o **catálogo anotado** com o progresso do jogador identificado **pelo path** (não pelo JWT).
- IF o jogador não tem progresso registrado -> retorna o catálogo com `progress=0` / `unlocked=false` (mesma semântica do `/me/achievements`).
- THE SYSTEM SHALL resolver o `playerID` exclusivamente do path; este endpoint **não** lê JWT.

### Catálogo (admin)
- WHEN `GET /admin/achievements` -> retorna o catálogo de conquistas (reusa `ListCatalog`).

### Invariante
- THE SYSTEM SHALL manter os endpoints `/admin/...` **sem auth** no MVP (uso interno/local). A barreira é a rede (não exposto publicamente).

## Contratos de Interface

### API HTTP (novas rotas)
| Método | Rota | Retorno |
|---|---|---|
| GET | `/admin/players/{playerID}/achievements` | `[{ id,name,description,target,progress,unlocked,unlockedAt }]` |
| GET | `/admin/achievements` | `[{ id,name,description,eventType,target }]` |

### Reuso (sem alteração)
- `AchievementService.ListForPlayer(ctx, playerID)` — já existe (transport novo só remove o escopo por JWT)
- `AchievementService.ListCatalog(ctx)` — já existe

## Casos de Teste Obrigatórios

- [ ] `GET /admin/players/{id}/achievements` -> catálogo anotado para o `id` do **path**
- [ ] Jogador sem progresso -> catálogo com `progress=0`, `unlocked=false`
- [ ] `GET /admin/achievements` -> retorna o catálogo
- [ ] O endpoint admin **não** exige header `Authorization` (diferente do `/me/achievements`)

## Fora do escopo

- **Auth** nos endpoints admin — interno/local no MVP; provável ADR futuro
- **Escrita / ações** (resetar progresso, conceder conquista) — read-only
- **Paginação / filtros / busca** no catálogo ou por jogador — incremental depois
- **Listar todos os jogadores** — é responsabilidade do identity (feature à parte, iteração 2)
