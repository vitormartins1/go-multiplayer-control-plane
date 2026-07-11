# Spec: 010-game-soft-delete

**Serviço:** game-registry
**Status:** Done (2026-07-08)
**Data:** 2026-07-08
**ADR:** 0025 (remoção de jogo — soft delete no registry)

## Contexto

Fecha o backlog "remover jogo no painel" (rename existe desde a spec `005`; o delete estava adiado aguardando o ADR da cascata). O ADR-0025 decidiu **soft delete no registry**: o dado por jogo vive em 6 stores sem transação entre eles, mas o registry é o **ponto de corte único** — matchmaking e session já dependem dele a cada fluxo (profilecache TTL 15s + snapshot obrigatório no `Create`/`OpenWorld`, ADR-0024/M1) e o console é jogo-cêntrico (sumir da listagem = sumir de tudo). Nenhum outro serviço muda.

Princípios da fatia:

- **Corte, não purge:** deletar = marcar `deleted_at`. O dado satélite (conquistas, saves, boards, rollups) fica órfão e inacessível; purge é evolução gated por evidência (Opção C do ADR).
- **Jogo deletado = jogo inexistente** para toda a API: 404 nas leituras/mutações, sem endpoint de undelete (recuperação = SQL manual).
- **Slug permanece reservado** (unicidade global, incluindo deletados): recriar "o mesmo jogo" exige slug novo — identidade nova de fato.
- **Sessões em voo terminam naturalmente:** sem force-close; o resultado de jogo deletado segue aceito pelos sinks (rollup órfão invisível).

## Comportamento

### Delete

- WHEN `DELETE /games/{id}` com o jogo vivo → THE SYSTEM SHALL marcar `deleted_at = now()` e retornar `204`.
- IF o jogo não existir **ou já estiver deletado** → `404` (**ErrGameNotFound**) — o delete é idempotente do ponto de vista do estado final, mas o segundo DELETE responde 404 (jogo já é "inexistente").

### Leituras e mutações passam a excluir deletados

- WHEN `GET /games` → THE SYSTEM SHALL listar somente jogos com `deleted_at IS NULL`.
- WHEN `GET /games/{id}`, `PUT /games/{id}` (rename), qualquer rota de `multiplayer-profile`/`multiplayer-profiles` sobre jogo deletado → `404` (**ErrGameNotFound**), idêntico a jogo inexistente.
- WHEN `PUT /games/{gameId}/players/{playerID}` (marca de presença) sobre jogo deletado → `404` (**ErrGameNotFound**). O identity chama best-effort (ADR-0019) — loga e segue; o login global não é afetado.
- WHEN `GET /games/{gameId}/players` sobre jogo deletado → lista vazia (base inacessível; mesma resposta de jogo sem players).
- WHEN `GET /players/{playerID}/games` (consulta reversa) → THE SYSTEM SHALL omitir jogos deletados.

### Criação e unicidade

- WHEN `POST /games` com slug de um jogo **deletado** → `409` (**ErrSlugTaken**) — a unicidade de slug continua global (constraint existente, sem mudança).

## Contratos de Interface

### API HTTP

| Método | Rota | Retorno |
|---|---|---|
| DELETE | `/games/{id}` | `204` \| `404` (inexistente ou já deletado) |
| (demais) | todas as existentes | inalteradas por fora; jogo deletado responde como inexistente |

### Tipos (domínio) — extensão

- `GameService` ganha: `Delete(ctx context.Context, id string) error` — `ErrGameNotFound` se ausente/já deletado.
- `GameRepository` ganha: `SoftDelete(ctx context.Context, id string) error` — mesma semântica.
- `Game` **não ganha campo** `DeletedAt`: jogo deletado nunca sai do repository (filtro na fonte), então o domínio não precisa representá-lo.
- Nenhum erro novo: `ErrGameNotFound` cobre todos os casos.

### Persistência (repository) — extensão

- Migration nova: `games.deleted_at TIMESTAMPTZ` (NULL = vivo). Nenhuma outra tabela muda.
- `List`, `FindByID`, `UpdateName`, `SetModes` passam a filtrar `deleted_at IS NULL`.
- Presença: `Upsert` só insere se o jogo estiver vivo (`ErrGameNotFound` caso contrário); `ListByGame` e `GamesByPlayer` passam a excluir jogos deletados.

## Modelo de Dados

```sql
ALTER TABLE games ADD COLUMN deleted_at TIMESTAMPTZ;  -- NULL = vivo
```

Sem índice novo: o volume de jogos é pequeno (catálogo, não série temporal) e todas as queries já buscam por PK ou varrem a tabela inteira.

## Casos de Teste Obrigatórios

- [ ] (service) `Delete` de jogo vivo → nil; segundo `Delete` → `ErrGameNotFound`; `Delete` de id inexistente → `ErrGameNotFound`
- [ ] (service) após `Delete`: `Get`/`Rename`/`SetMode`/`GetMultiplayerProfile` → `ErrGameNotFound`; `List` não inclui o deletado
- [ ] (service) presença: `Mark` em jogo deletado → `ErrGameNotFound`
- [ ] (repo, integração) `SoftDelete` marca `deleted_at` e preserva a linha; `FindByID`/`List`/`UpdateName`/`SetModes` ignoram deletados; re-`Create` com o mesmo slug → `ErrSlugTaken` — testcontainers
- [ ] (repo, integração) presença: `Upsert` em jogo deletado → `ErrGameNotFound`; `GamesByPlayer` omite deletados; `ListByGame` de deletado → vazio
- [ ] (transport) `DELETE /games/{id}` → 204; repetido → 404; exige internal key
- [ ] (transport) rotas existentes sobre jogo deletado → 404 (um caso representativo basta: `GET /games/{id}`)

## Fora do escopo

- **Purge do dado satélite** (Opção C do ADR-0025) — evolução gated por evidência (custo de storage/LGPD).
- **Undelete via API** — recuperação é `UPDATE games SET deleted_at = NULL` manual.
- **Force-close das sessões abertas** do jogo deletado — terminam naturalmente; o board Rooms as mostra até lá (fiel à realidade do data plane).
- **BFF e console** — specs companheiras `admin-bff/018-game-delete-passthrough` e `admin-ui/038-game-delete`.
