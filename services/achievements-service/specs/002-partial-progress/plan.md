# Plan: 002-partial-progress
**Spec:** specs/002-partial-progress/spec.md
**Data:** 2026-06-28

## Abordagem técnica

Evolução incremental da 001, sem novos pacotes nem mudança de transporte. O coração é mover o desbloqueio de "insere se não existe" (001) para "incrementa e desbloqueia ao atingir a meta" (002), mantendo a atomicidade no banco. A 001 vira o caso `target = 1` — sem código especial.

A linha de `player_achievements` passa a representar **progresso** (não só desbloqueio): existe assim que o jogador acumula a primeira ocorrência; `unlocked_at NULL` = em progresso; preenchido = desbloqueada.

## Banco (ADR-0015)

Postgres próprio do achievements (já existente). Migration **002** altera o schema da 001 — adicionada ao initdb do `achievements-postgres` e aos `WithInitScripts` do teste de integração (rodam 001 → 002 em ordem).

## Arquivos a criar ou modificar

- `migrations/002_add_progress.sql` — **novo:** `ALTER` em `achievements` (+`target`) e `player_achievements` (+`progress`, `unlocked_at` nullable) + seed de uma conquista progressiva
- `internal/domain/achievement.go` — `Achievement` +`Target`; `PlayerAchievement` +`Progress`; novo `PlayerProgress`; Ports: `Unlock`→`RecordProgress`, `ListPlayerUnlocks`→`ListPlayerProgress` (remove a struct `Unlock`)
- `internal/service/achievement_service.go` — `RecordEvent` usa `RecordProgress(target)`; `ListForPlayer` usa o progresso (clampa `progress` ao `target`)
- `internal/service/achievement_service_test.go` — atualizar mock + casos (incrementa sem desbloquear; atinge meta; no-op pós-desbloqueio; target=1)
- `internal/repository/postgres_achievement.go` — `RecordProgress` (UPSERT atômico), `ListPlayerProgress`, `scan` com `target`
- `internal/repository/postgres_achievement_test.go` — `WithInitScripts` 001+002; casos de progresso/desbloqueio
- `internal/transport/http_handler.go` — DTOs com `progress`/`target`
- `internal/transport/http_handler_test.go` — atualizar fakes + asserts de progresso
- `infra/docker/docker-compose.yml` — montar `002_add_progress.sql` no initdb do `achievements-postgres`
- `services/achievements-service/api/achievements.http` — exemplo de progresso (enviar `match_won` N vezes)

## Decisões técnicas e trade-offs

- **`RecordProgress` atômico (sem corrida), via UPSERT condicional:**

  ```sql
  INSERT INTO player_achievements (player_id, achievement_id, progress, unlocked_at)
  VALUES ($1, $2, 1, CASE WHEN 1 >= $3 THEN $4 ELSE NULL END)
  ON CONFLICT (player_id, achievement_id) DO UPDATE
    SET progress    = LEAST(player_achievements.progress + 1, $3),
        unlocked_at = COALESCE(
                        player_achievements.unlocked_at,
                        CASE WHEN player_achievements.progress + 1 >= $3 THEN $4 END)
    WHERE player_achievements.unlocked_at IS NULL
  RETURNING unlocked_at;
  ```

  `$3 = target`, `$4 = now`. Interpretação do retorno em Go (`unlocked_at` como `*time.Time`):
  - **0 linhas** (`pgx.ErrNoRows`) → o `WHERE` barrou: já estava desbloqueada → `justUnlocked = false`.
  - **1 linha, `unlocked_at = nil`** → progrediu, ainda não atingiu a meta → `false`.
  - **1 linha, `unlocked_at != nil`** → atingiu a meta **nesta** chamada → `justUnlocked = true`.

  Tudo numa instrução: incrementa, trava em `target` (`LEAST`), só mexe enquanto não desbloqueada (`WHERE`) e nunca regride o `unlocked_at` (`COALESCE`). Mesma filosofia da 001 (atomicidade no store, GO-LEARNINGS #01), agora com contagem.

- **`target = 1` reaproveita o mesmo caminho:** a `first_win` (target 1) desbloqueia no 1º evento; a nova `ten_wins` (target 10, mesmo `event_type` `match_won`) progride — um único `match_won` afeta as duas, exercitando "N conquistas por evento".
- **Resposta aditiva:** `GET /me/achievements` e `/achievements` ganham `progress`/`target`; clientes da 001 continuam válidos.

## Ordem de implementação

1. `migrations/002_add_progress.sql` + montar no compose e no testcontainer
2. `domain` (Target, PlayerProgress, Ports) — quebra o build de propósito, guiando o resto
3. `service` (RecordProgress, progresso no ListForPlayer) + unit tests
4. `repository` (UPSERT, ListPlayerProgress) + integration tests
5. `transport` (DTOs progress/target) + httptest
6. `go test ./... -race` + integração verdes; `go build ./...`
7. Validar no Docker: enviar `match_won` 1..10× e ver `ten_wins` progredindo até desbloquear; `first_win` no 1º

## Riscos e pontos de atenção

- **Migration sobre tabela existente:** localmente o `achievements-postgres` recria do zero (sem volume nomeado), então initdb roda 001→002. Em ambiente com volume persistente a 002 não roda sozinha — fora do escopo local-first (migrations versionadas reais virão com o deploy).
- **`progress` exibido vs interno:** o `LEAST` trava o valor persistido em `target`; o service ainda clampa na leitura por garantia.
- **testcontainers exige Docker.**
