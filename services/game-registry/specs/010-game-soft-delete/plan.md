# Plan: 010-game-soft-delete

**Spec:** `spec.md` · **ADR:** 0025

## Decisões técnicas

1. **Filtro na fonte: o repository nunca devolve jogo deletado.**
   `List`, `FindByID`, `UpdateName` e `SetModes` ganham `AND deleted_at IS NULL` / `WHERE deleted_at IS NULL`. Consequência: **service e transport existentes não mudam nada** — jogo deletado vira `ErrGameNotFound` pelos caminhos que já existem (Get, Rename, modos, alias de perfil). O domínio não representa o estado "deletado" (`Game` sem campo `DeletedAt`): quem está deletado simplesmente não existe acima do repository.

2. **`SoftDelete` por `UPDATE` condicional.**
   `UPDATE games SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`; `RowsAffected == 0` → `ErrGameNotFound` (cobre inexistente **e** já deletado com uma query só, sem read-modify-write nem corrida).

3. **Presença: gate no SQL, não no service.**
   - `Upsert` vira `INSERT ... SELECT $1,$2,$3,now() WHERE EXISTS (SELECT 1 FROM games WHERE id = $1 AND deleted_at IS NULL) ON CONFLICT ... DO UPDATE`; `RowsAffected == 0` → `ErrGameNotFound` (a doc do Port já promete esse erro para "jogo inexistente" — a semântica só passa a incluir deletado; a FK continua cobrindo id inexistente). Sem SELECT prévio = sem corrida com o delete.
   - `GamesByPlayer` já faz JOIN com `games`: ganha `AND g.deleted_at IS NULL`.
   - `ListByGame` ganha JOIN com `games` vivo → jogo deletado responde lista vazia (mesma forma de jogo sem players; o console nem chega lá, pois o jogo some da biblioteca).

4. **Slug reservado de graça.**
   A constraint UNIQUE de `slug` é global e não muda — `Create` em slug de jogo deletado já devolve `ErrSlugTaken` pelo caminho existente. Só documentar (spec) e testar.

5. **Migration `006_add_deleted_at.sql`** — um `ALTER TABLE` só. **Gotcha operacional:** migrations rodam em volume novo; validar com `docker compose rm -sfv game-registry game-registry-postgres` (4ª recriação do volume — padrão conhecido).

6. **Transport: uma rota nova.**
   `DELETE /games/{id}` → `h.delete` → `games.Delete` → 204 / `ErrGameNotFound`→404 `game_not_found` (mapeamento existente). Atrás da internal key como todo o mux (ADR-0018).

## Arquivos

| Arquivo | Mudança |
|---|---|
| `migrations/006_add_deleted_at.sql` | `ALTER TABLE games ADD COLUMN deleted_at TIMESTAMPTZ` |
| `internal/domain/game.go` | `GameService.Delete`; `GameRepository.SoftDelete` |
| `internal/service/game_service.go` | `Delete` (repassa ao repo; sem regra extra) |
| `internal/repository/postgres_game.go` | `SoftDelete`; filtro em `List`/`FindByID`/`UpdateName` |
| `internal/repository/postgres_profile.go` | filtro em `SetModes` |
| `internal/repository/postgres_presence.go` | gate no `Upsert`; filtro em `GamesByPlayer`/`ListByGame` |
| `internal/transport/http_handler.go` | rota `DELETE /games/{id}` + handler |
| testes dos 3 níveis | mock ganha `SoftDelete`; casos da spec |

## Riscos / atenção

- **Staleness a jusante é aceito por design:** matchmaking pode formar match por até 15s (TTL do profilecache) após o delete — a sessão morre no `Create` (snapshot falha no registry). Não tentar "avisar" o matchmaking; é o mecanismo do ADR.
- **Identity intocado:** o `Mark` best-effort já engole erro com log (ADR-0019) — o 404 novo não quebra o login-into-game. Confirmar no teste manual da collection `.http`, não com mudança de código.
- **Mocks:** `GameRepository` ganha método novo → atualização mecânica dos mocks de service.
- `processed_*`/rollups/boards órfãos ficam intactos por definição (ADR-0025) — nenhum teste deve esperar limpeza.
