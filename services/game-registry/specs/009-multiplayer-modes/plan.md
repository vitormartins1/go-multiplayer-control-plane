# Plan: 009-multiplayer-modes

**Spec:** `spec.md` (aprovada 2026-07-07) · **ADR:** 0024 · **Fatia:** M0 (lado game-registry)

## Decisões técnicas

1. **Uma única operação de escrita no repo: `SetModes` (replace da coleção).**
   O Port `GameRepository.UpsertMultiplayerProfile` **sai**; entra `SetModes(ctx, gameID, modes []MultiplayerMode)` que serializa o mapa inteiro e faz um `UPDATE` da coluna JSONB. O service faz read-modify-write (FindByID → muta a slice → SetModes). **Trade-off aceito:** last-write-wins em edições concorrentes de modos diferentes — o único escritor é o console admin (mesma exposição do PUT atual). Evita callback-under-tx/locks que o uso não justifica.

2. **Formato v2 + upgrade lazy no decode (sem migration SQL).**
   Armazenado: `{"modes": {"<modeId>": {displayName?, archetype, topology, params}}}` (`storedMode` = `storedProfile` + `displayName`). O decode sonda o topo: chave `modes` presente → v2; senão, objeto v1 (`archetype` no topo) → interpretado como `{modes: {default: v1}}` com `displayName` vazio. Todo write persiste v2. Os `storedQueueArena`/`storedBattleRoyale`/etc. e o `decodeProfile`/`encodeProfile` existentes são **reusados** (viram encode/decode de um modo).

3. **Compat interna: `Game.MultiplayerProfile` continua sendo o modo `default`.**
   `FindByID` carrega `Game.MultiplayerModes []MultiplayerMode` (ordenada: `default` primeiro, demais por `modeId` asc — ordenação no decode, determinística) e mantém `Game.MultiplayerProfile` apontando para o `Profile` do `default` (ou nil). Consumidores internos (transport `get`, service legado) não mudam de fonte.

4. **Service: validação de perfil extraída e reusada por modo.**
   O corpo do `SetMultiplayerProfile` atual (arquétipo/topologia/params por arquétipo) vira `validateProfile(p) error`; `SetMode` chama `validateProfile` + regras novas: `modeID` casa `^[a-z][a-z0-9-]{0,31}$` (`ErrInvalidModeID`), `displayName` trimmed ≤ 60 (violação → `ErrInvalidProfileParams` — não vale um erro novo), cap **16 modos** na criação (`ErrTooManyModes`), `DeleteMode("default")` → `ErrDefaultModeProtected`.

5. **Legado delega para o `default` preservando o `displayName`.**
   `SetMultiplayerProfile` = `SetMode(gameID, "default", <displayName existente, se houver>, p)`; `GetMultiplayerProfile` = `Profile` do modo `default` ou `ErrProfileNotFound`. Contrato HTTP das specs 006–008 intacto (mesmos códigos; corpo sem `displayName`).

6. **Transport.**
   Rotas novas no `Register` (ServeMux 1.22): `GET /games/{id}/multiplayer-profiles`, `PUT|GET|DELETE /games/{id}/multiplayer-profiles/{modeId}`. `modeDTO` embute `profileDTO` (flatten JSON) + `modeId`/`displayName`. Corpo do PUT = `{displayName?, archetype, topology, params}` (modeId vem do path; `DisallowUnknownFields` mantido). `GET /games/{id}` ganha `multiplayerModes: [{modeId, displayName, archetype}]` (lista, `[]` quando vazio). Mapeamentos novos: `ErrInvalidModeID`→400 `invalid_mode_id`, `ErrModeNotFound`→404 `mode_not_found`, `ErrDefaultModeProtected`→409 `default_mode_protected`, `ErrTooManyModes`→400 `too_many_modes`.

## Arquivos

| Arquivo | Mudança |
|---|---|
| `internal/domain/game.go` | `MultiplayerMode`; `Game.MultiplayerModes`; `GameService` +4 métodos; `GameRepository.SetModes` (remove `UpsertMultiplayerProfile`) |
| `internal/domain/errors.go` | `ErrInvalidModeID`, `ErrModeNotFound`, `ErrDefaultModeProtected`, `ErrTooManyModes` |
| `internal/service/profile_service.go` | `validateProfile` extraída; `SetMode`/`GetMode`/`ListModes`/`DeleteMode`; legado delega; regex/cap |
| `internal/repository/postgres_profile.go` | `storedMode`/`storedModes`; `encodeModes`/`decodeModes` (v2 + lazy v1); `SetModes` |
| `internal/repository/postgres_game.go` | `FindByID` popula `MultiplayerModes` + `MultiplayerProfile` (default) |
| `internal/transport/http_handler.go` | rotas, `modeDTO`, summary no `get`, error mapping |
| testes dos 3 níveis | mocks atualizados (`SetModes`), casos da spec |

## Riscos / atenção

- **Mocks e testes existentes** referenciam `UpsertMultiplayerProfile` (service unit + repo integração): atualização mecânica para `SetModes` + `FindByID` no arrange; os casos das specs 006–008 continuam passando via caminho legado (prova da não-regressão).
- Ordem das rotas no ServeMux: literais `multiplayer-profile` e `multiplayer-profiles` não conflitam (segmentos distintos).
- `GET /games` (lista) segue sem carregar perfil/modos (sem mudança de custo).
