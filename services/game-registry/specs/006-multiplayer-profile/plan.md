# Plan: 006-multiplayer-profile

**Serviço:** game-registry · **Spec:** [spec.md](./spec.md) · **ADR:** 0023

## Objetivo

Adicionar ao game-registry a persistência/validação/exposição do **perfil de multiplayer** (arquétipo `queue_arena`, topologia `dedicated`), reusando os padrões existentes (hexagonal — ADR-0002; pgx; internal key — ADR-0018). Sem tocar matchmaking/session.

## Decisões técnicas

### D1 — Persistência: coluna `JSONB` na tabela `games` (não tabela dedicada)

- **Escolha:** `ALTER TABLE games ADD COLUMN multiplayer_profile JSONB` (NULL = sem perfil). O perfil é 1:1 opcional com o jogo e vive na mesma tabela.
- **Por quê:** relação 1:1 opcional → coluna é mais simples que uma tabela com FK/join; `JSONB` acomoda os **params polimórficos** dos arquétipos futuros (`battle_royale`, etc.) **sem nova migration**. `FindByID` já lê `games`, então o perfil vem junto sem query extra.
- **Alternativa descartada:** tabela `game_multiplayer_profiles(game_id PK/FK, …)` — mais explícita, mas adiciona join e migration por arquétipo; ganho nulo no MVP 1:1.
- **Formato no JSONB:** o perfil inteiro `{ "archetype", "topology", "params": { … } }`. `archetype`/`topology` ficam no topo para eventual consulta futura; `params` é o objeto do arquétipo.

### D2 — Params polimórficos no domínio: **ponteiro por arquétipo** (tagged union pragmática)

- **Escolha:**
  ```go
  type MultiplayerProfile struct {
      Archetype string            // "queue_arena"
      Topology  string            // "dedicated"
      QueueArena *QueueArenaParams // não-nil sse Archetype == "queue_arena"; nil caso contrário
  }
  ```
  Novos arquétipos adicionam um campo-ponteiro (`BattleRoyale *BattleRoyaleParams`), mantendo type-safety e (de)serialização JSON natural. **Invariante** (validada no service): exatamente o ponteiro do `Archetype` declarado é não-nil.
- **Por quê:** idiomático e explícito em Go para "union type opcional"; evita `interface{}` + type-switch mágico e evita `any` no DB. Legível para quem vem de C#/.NET (é o análogo de um discriminated union).
- **Alternativa descartada:** interface marcadora `ArchetypeParams` + type switch — mais "OO", porém exige discriminador manual na (de)serialização sem ganho no MVP.

### D3 — Leitura reusa `FindByID` (perfil embutido no `Game`); escrita é operação própria

- `domain.Game` ganha `MultiplayerProfile *MultiplayerProfile` (nil = sem perfil). `FindByID` passa a ler a coluna e popular; `List` **não** lê o perfil (mantém `selectCols` atual → DTO da lista inalterado, atende "lista não infla").
- **Leitura do endpoint dedicado** `GET /games/{id}/multiplayer-profile` reusa `FindByID`: jogo ausente → `ErrGameNotFound`; jogo presente com `MultiplayerProfile == nil` → `ErrProfileNotFound`. **Não** criamos `FindMultiplayerProfile` no repo (simplificação vs. a sugestão da spec — a spec rege comportamento, não assinaturas).
- **Escrita:** `UpsertMultiplayerProfile(ctx, gameID, p)` = `UPDATE games SET multiplayer_profile = $2 WHERE id = $1`; `RowsAffected() == 0` → `ErrGameNotFound` (mesmo padrão de `UpdateName`).

### D4 — Validação no `service` (regra de negócio pura), antes do DB

- `SetMultiplayerProfile` valida na ordem: `archetype` (== `queue_arena`, senão `ErrUnsupportedArchetype`) → `topology` (== `dedicated`, senão `ErrUnsupportedTopology`) → params (`validateQueueArena`, senão `ErrInvalidProfileParams`) → só então `repo.UpsertMultiplayerProfile` (que valida existência do jogo). Validação barata antes de tocar o banco.
- `validateQueueArena` reproduz as regras condicionais da spec (`structure`, `winCondition`, `matchStructure`, `scoringStats` via regex `^[a-z][a-z0-9_]*$`). Enums e regex como `var` de pacote, no estilo de `slugRE`/`maxNameLen`.

## Modelo de dados

`migrations/005_add_multiplayer_profile.sql`:
```sql
-- 005: perfil de multiplayer por jogo (ADR-0023, spec 006).
-- 1:1 opcional; NULL = jogo sem perfil configurado. JSONB acomoda params
-- polimórficos por arquétipo (queue_arena no MVP).
ALTER TABLE games ADD COLUMN multiplayer_profile JSONB;
```
Sem backfill: jogos existentes ficam com `NULL` (= sem perfil), coerente com "perfil opcional".

## Mudanças por camada (arquivos)

| Arquivo | Mudança |
|---|---|
| `internal/domain/game.go` | tipos `MultiplayerProfile`, `QueueArenaParams`; campo `MultiplayerProfile *MultiplayerProfile` em `Game`; `GameService` += `SetMultiplayerProfile`/`GetMultiplayerProfile`; `GameRepository` += `UpsertMultiplayerProfile` |
| `internal/domain/errors.go` | `ErrUnsupportedArchetype`, `ErrUnsupportedTopology`, `ErrInvalidProfileParams`, `ErrProfileNotFound` |
| `internal/service/profile_service.go` (novo) | `SetMultiplayerProfile`, `GetMultiplayerProfile`, `validateQueueArena` + enums/regex. Métodos no mesmo `*service` (implementa `GameService`) |
| `internal/repository/postgres_game.go` | `UpsertMultiplayerProfile`; `FindByID` lê `multiplayer_profile` e popula (encode/decode JSON ↔ struct); `List` inalterado |
| `internal/transport/http_handler.go` | rotas `PUT`/`GET /games/{id}/multiplayer-profile`; DTOs `profileDTO`/`queueArenaParamsDTO`; `get` de jogo estendido para incluir `multiplayerProfile` (perfil ou `null`); `list` inalterado; `writeDomainError` += 4 erros (400/400/400/404) |
| `migrations/005_add_multiplayer_profile.sql` (novo) | coluna JSONB |
| `cmd/server/main.go` | **sem mudança** — wire-up e guard (`/games`) já cobrem |

**Nota DTO (D3):** `get` usa um DTO que embute `gameDTO` + `multiplayerProfile` **sem** `omitempty` (serializa `null` quando ausente); `list` mantém o `gameDTO` atual (sem o campo). `decode` segue com `DisallowUnknownFields`.

## Testes (mapeiam os casos da spec)

- **service** (`profile_service_test.go`, `testify/mock` do repo): upsert válido; archetype/topology não suportados; cada regra condicional de `queue_arena` inválida; `scoringStats` bom/ruim; `ErrGameNotFound` propagado do repo. Cobertura ≥ 80% (regra do projeto).
- **repo** (`postgres_game_test.go`, testcontainers): upsert round-trip; reconfiguração substitui; `FindByID` popula o perfil; jogo sem perfil → `MultiplayerProfile == nil`; upsert em id inexistente → `ErrGameNotFound`.
- **transport** (`http_handler_test.go`, `httptest`): `PUT` 200/400/404; `GET /games/{id}/multiplayer-profile` 200/404; `GET /games/{id}` inclui `multiplayerProfile` (perfil e `null`); `GET /games` **não** inclui.

## Observabilidade

Sem trabalho novo: as rotas entram automaticamente nas métricas RED (`metrics.Wrap`) e no span por rota (`RouteSpanName`); mesmo banco → sem novo exporter. Só conferir os nomes de rota no dashboard após implementar (ADR-0006/0014).

## Riscos / fora do escopo

- **Risco baixo:** mudança aditiva (coluna nullable, endpoints novos); não altera contratos existentes de `/games`.
- Consumo do perfil (matchmaking/session) e **resultado polimórfico** — próximas fatias (fora daqui).
- (De)serialização do `params` polimórfico exigirá cuidado quando entrar o 2º arquétipo; no MVP há só um caminho (`queue_arena`).
